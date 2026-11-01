package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/expvintl/scopex/internal/model"
)

// 复现线上现象：任务运行中点“详情”，前端每 1.8 秒轮询一次
// /api/tasks/:id/logs?after=<已读游标>。增量查询没有新日志时 store 返回 nil
// 切片，json.Marshal 会把它编码成 null，前端再读 logs.length 就抛
// "Cannot read properties of null (reading 'length')"，整个详情页报错。
// 这里锁死契约：logs 永远是数组。
func TestTaskLogsNeverNull(t *testing.T) {
	ctx := context.Background()
	st, cfg := setup(t)
	h := newTestServer(t, st, cfg)
	token := login(t, h, testAdminPass)

	task := &model.Task{
		Name:      "logs-null",
		Targets:   []string{"127.0.0.1"},
		Ports:     "22",
		Tool:      "native",
		Status:    model.TaskRunning,
		CreatedBy: "admin",
		CreatedAt: time.Now(),
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatalf("写入任务失败: %v", err)
	}
	// 任务刚入队时只有一条“开始”日志，之后进度阶段长时间没有新日志。
	logs := []model.TaskLog{{Seq: 1, Level: "info", Msg: "开始扫描", At: time.Now()}}
	if err := st.AppendLogs(ctx, task.ID, logs); err != nil {
		t.Fatalf("写入日志失败: %v", err)
	}

	path := "/api/tasks/" + task.ID.Hex() + "/logs"
	w := call(t, h, http.MethodGet, path+"?after=0", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("拉取日志失败: %d %s", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); !strings.Contains(body, `"logs":[`) {
		t.Fatalf("logs 必须是数组，实际: %s", body)
	}

	// 游标追平后再拉一次：这是用户点开详情页后立刻会走到的那一次请求。
	w = call(t, h, http.MethodGet, path+"?after=1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("增量拉取失败: %d %s", w.Code, w.Body.String())
	}
	body := strings.TrimSpace(w.Body.String())
	if strings.Contains(body, `"logs":null`) {
		t.Fatalf("没有新日志时 logs 不能是 null: %s", body)
	}
	if !strings.Contains(body, `"logs":[]`) {
		t.Fatalf("没有新日志时 logs 应该是空数组: %s", body)
	}
}
