// Package logging 负责把日志分成两条通道：
//
//   - 屏幕只输出警告及以上，外加少量用 Status/Notice 显式打印的关键信息，
//     例如首次安装生成的管理员密码，避免被大量明细淹没；
//   - 文件记录全部级别（默认 info），包含每次 HTTP 请求等明细，便于事后排查。
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Options 描述日志输出策略。
type Options struct {
	// ConsoleLevel 是打印到屏幕的最低级别；细节日志请用文件。
	ConsoleLevel slog.Level
	// File 为空表示不写日志文件。
	File      string
	FileLevel slog.Level
	// MaxMB 是单个日志文件的大小上限，Backups 是保留的历史文件数量。
	MaxMB   int
	Backups int
}

var (
	stdout   io.Writer = &syncWriter{w: os.Stdout}
	fileOut  *rotateWriter
	clockNow = time.Now
)

// Setup 初始化全局 slog 与日志文件，返回关闭函数。应在进程启动时调用一次。
func Setup(o Options) (func() error, error) {
	if o.MaxMB <= 0 {
		o.MaxMB = 20
	}
	if o.Backups <= 0 {
		o.Backups = 3
	}

	handlers := []slog.Handler{
		slog.NewTextHandler(stdout, &slog.HandlerOptions{
			Level: o.ConsoleLevel,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				// 屏幕上时间只用时分秒，其余字段保持 key=value 便于阅读。
				if a.Key == slog.TimeKey {
					if t, ok := a.Value.Any().(time.Time); ok {
						return slog.String("time", t.Format("15:04:05"))
					}
				}
				return a
			},
		}),
	}

	if o.File != "" {
		rw, err := openRotating(o.File, int64(o.MaxMB)<<20, o.Backups)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件 %s: %w", o.File, err)
		}
		fileOut = rw
		handlers = append(handlers, slog.NewTextHandler(rw, &slog.HandlerOptions{Level: o.FileLevel}))
	}

	slog.SetDefault(slog.New(fanout{handlers: handlers}))
	if fileOut != nil {
		return fileOut.Close, nil
	}
	return func() error { return nil }, nil
}

// FilePath 返回当前日志文件路径，未启用时为空。
func FilePath() string {
	if fileOut == nil {
		return ""
	}
	return fileOut.path
}

// Status 打印一条始终可见的运行状态，同时写入日志文件。
// 只用于少量真正需要用户当场看到的信息，明细日志请走 slog。
func Status(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	stamp := clockNow()
	fmt.Fprintf(stdout, "[%s] %s\n", stamp.Format("15:04:05"), line)
	writeFile(stamp, line)
}

// Notice 用醒目的方框提示关键信息（例如初始管理员密码），屏幕上不会被忽略。
func Notice(title string, pairs ...[2]string) {
	const width = 62
	bar := strings.Repeat("=", width)

	var b strings.Builder
	b.WriteString("\n" + bar + "\n")
	b.WriteString("  " + title + "\n")
	for _, p := range pairs {
		if p[0] == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(fmt.Sprintf("    %s: %s\n", p[0], p[1]))
	}
	b.WriteString(bar + "\n\n")

	block := b.String()
	fmt.Fprint(stdout, block)
	writeFile(clockNow(), strings.TrimRight(block, "\n"))
}

func writeFile(stamp time.Time, line string) {
	if fileOut == nil || line == "" {
		return
	}
	_, _ = fmt.Fprintf(fileOut, "%s %s\n", stamp.Format(time.RFC3339), line)
}

// ParseLevel 解析 debug/info/warn/error，非法值回退到 fallback。
func ParseLevel(s string, fallback slog.Level) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return fallback
}

// syncWriter 串行化写入，避免 slog 与 Notice 的输出互相穿插。
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// fanout 把同一条记录分发给多个 handler。
type fanout struct{ handlers []slog.Handler }

func (f fanout) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if e := h.Handle(ctx, r.Clone()); e != nil {
				err = e
			}
		}
	}
	return err
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return f
	}
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return fanout{handlers: next}
}

func (f fanout) WithGroup(name string) slog.Handler {
	if name == "" {
		return f
	}
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return fanout{handlers: next}
}

// rotateWriter 是按大小轮转的日志文件写入器：写满后改名为 .1，保留 Backups 份历史。
type rotateWriter struct {
	mu      sync.Mutex
	path    string
	max     int64
	backups int
	f       *os.File
	size    int64
}

func openRotating(path string, max int64, backups int) (*rotateWriter, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	w := &rotateWriter{path: path, max: max, backups: backups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotateWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	return nil
}

func (w *rotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.max > 0 && w.size+int64(len(p)) > w.max {
		w.rotate()
	}
	if w.f == nil {
		return 0, os.ErrClosed
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotateWriter) rotate() {
	_ = w.f.Close()
	w.f = nil
	for i := w.backups; i > 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i-1), fmt.Sprintf("%s.%d", w.path, i))
	}
	if w.backups > 0 {
		_ = os.Rename(w.path, w.path+".1")
	} else {
		_ = os.Remove(w.path)
	}
	if err := w.open(); err != nil {
		w.size = 0
	}
}

func (w *rotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
