package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capture 接管屏幕输出并启用临时日志文件，返回屏幕缓冲与文件内容读取函数。
func capture(t *testing.T, console slog.Level) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	path := filepath.Join(t.TempDir(), "logs", "scopex.log")

	oldOut, oldFile := stdout, fileOut
	stdout = &buf
	stop, err := Setup(Options{ConsoleLevel: console, File: path, FileLevel: slog.LevelInfo, MaxMB: 1, Backups: 2})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() {
		_ = stop()
		stdout, fileOut = oldOut, oldFile
	})
	return &buf, path
}

// TestConsoleKeepsKeyLinesAndFileKeepsDetails 覆盖核心诉求：
// 屏幕只留关键信息（状态行、方框提示、警告），明细例如每次 HTTP 请求只进文件。
func TestConsoleKeepsKeyLinesAndFileKeepsDetails(t *testing.T) {
	console, path := capture(t, slog.LevelWarn)

	Status("监听 %s · 数据库 %s", ":8088", "scopex")
	Notice("首次安装：已创建管理员账号",
		[2]string{"用户名", "admin"},
		[2]string{"密码", "s3cret-pass"},
		[2]string{"提示", "登录后请立即修改密码"},
	)
	slog.Info("http", "method", "GET", "path", "/api/health", "status", 200)
	slog.Warn("磁盘空间不足")

	screen := console.String()
	for _, want := range []string{"监听 :8088 · 数据库 scopex", "=====", "密码: s3cret-pass", "磁盘空间不足"} {
		if !strings.Contains(screen, want) {
			t.Errorf("屏幕输出缺少 %q\n实际:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "/api/health") {
		t.Errorf("屏幕上不应出现 HTTP 明细\n实际:\n%s", screen)
	}

	if got := FilePath(); got != path {
		t.Fatalf("FilePath() = %q, 期望 %q", got, path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取日志文件: %v", err)
	}
	file := string(body)
	for _, want := range []string{"监听 :8088 · 数据库 scopex", "s3cret-pass", "msg=http", "path=/api/health", "磁盘空间不足"} {
		if !strings.Contains(file, want) {
			t.Errorf("日志文件缺少 %q\n实际:\n%s", want, file)
		}
	}
}

// TestFileDisabled 保证 File 为空时不写文件，也不影响屏幕。
func TestFileDisabled(t *testing.T) {
	var buf bytes.Buffer
	oldOut, oldFile := stdout, fileOut
	stdout, fileOut = &buf, nil
	t.Cleanup(func() { stdout, fileOut = oldOut, oldFile })

	stop, err := Setup(Options{ConsoleLevel: slog.LevelInfo})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer func() { _ = stop() }()

	Status("仅屏幕")
	if FilePath() != "" {
		t.Fatalf("未配置文件时 FilePath() 应为空，实际 %q", FilePath())
	}
	if !strings.Contains(buf.String(), "仅屏幕") {
		t.Fatalf("状态行应打印到屏幕，实际:\n%s", buf.String())
	}
}

// TestRotateBySize 检查写满后轮转并保留指定份数的历史文件。
func TestRotateBySize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scopex.log")
	w, err := openRotating(path, 64, 2)
	if err != nil {
		t.Fatalf("openRotating: %v", err)
	}
	defer func() { _ = w.Close() }()

	for i := 0; i < 5; i++ {
		if _, err := w.Write([]byte(strings.Repeat("x", 40) + "\n")); err != nil {
			t.Fatalf("写入第 %d 条: %v", i, err)
		}
	}

	for _, name := range []string{path, path + ".1", path + ".2"} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("轮转后应存在 %s: %v", name, err)
		}
		if info.Size() > 64 {
			t.Errorf("%s 超过单文件上限: %d", name, info.Size())
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Errorf("只保留 2 份历史，不应出现 .3")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		" INFO ":  slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"unknown": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in, slog.LevelInfo); got != want {
			t.Errorf("ParseLevel(%q) = %v, 期望 %v", in, got, want)
		}
	}
}
