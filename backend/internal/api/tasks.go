package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/expvintl/scopex/internal/i18n"
	"github.com/expvintl/scopex/internal/model"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/util"
)

func (s *Server) createTask(c *gin.Context) {
	var req struct {
		Name    string   `json:"name"`
		Targets []string `json:"targets"`
		Ports   string   `json:"ports"`
		Rate    int      `json:"rate"`
		Tool    string   `json:"tool"`
		Deep    bool     `json:"deep"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	targets := make([]string, 0, len(req.Targets))
	for _, t := range req.Targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, err := util.ParseTarget(t); err != nil {
			failf(c, http.StatusBadRequest, "无效的目标 %s：%s", t, i18n.Message(lang(c), err))
			return
		}
		targets = append(targets, t)
	}
	if len(targets) == 0 {
		fail(c, http.StatusBadRequest, "请至少填写一个扫描目标")
		return
	}
	if len(targets) > 32 {
		fail(c, http.StatusBadRequest, "单个任务最多 32 个目标段")
		return
	}
	ports := strings.TrimSpace(req.Ports)
	if ports == "" {
		ports = defaultPorts(c.Request.Context(), s)
	}
	if _, err := scanner.ParsePorts(ports); err != nil {
		failf(c, http.StatusBadRequest, "端口格式错误：%s", i18n.Message(lang(c), err))
		return
	}
	rate := req.Rate
	if rate <= 0 {
		rate = 1000
	}
	if rate > s.cfg.Scanner.MaxRate {
		rate = s.cfg.Scanner.MaxRate
	}
	switch req.Tool {
	case "", "auto":
		req.Tool = "auto"
	case "native", "masscan", "nmap":
	default:
		fail(c, http.StatusBadRequest, "不支持的扫描引擎")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "scan-" + time.Now().Format("0102-1504")
	}
	now := time.Now()
	task := &model.Task{
		Name:      name,
		Targets:   targets,
		Ports:     ports,
		Rate:      rate,
		Tool:      req.Tool,
		Deep:      req.Deep,
		Status:    model.TaskPending,
		Phase:     "queued",
		CreatedBy: currentUser(c).Username,
		Lang:      string(lang(c)),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateTask(c.Request.Context(), task); err != nil {
		failf(c, http.StatusInternalServerError, "创建任务失败：%s", i18n.Message(lang(c), err))
		return
	}
	if !s.engine.Submit(task.ID) {
		s.store.UpdateTask(c.Request.Context(), task.ID, bson.M{
			"status": model.TaskFailed, "error": "任务队列已满", "finished_at": time.Now(), "phase": "failed",
		})
		fail(c, http.StatusServiceUnavailable, "任务队列已满，请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": task})
}

func (s *Server) listTasks(c *gin.Context) {
	page, size := pagination(c, 20, 100)
	status := c.Query("status")
	items, total, err := s.store.ListTasks(c.Request.Context(), status, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询任务失败")
		return
	}
	if items == nil {
		items = []model.Task{}
	}
	for i := range items {
		normalizeTask(&items[i])
	}
	okList(c, items, total, page, size)
}

// normalizeTask 保证 JSON 中集合字段始终为数组，避免前端处理 null。
func normalizeTask(t *model.Task) {
	if t.Targets == nil {
		t.Targets = []string{}
	}
}

func (s *Server) getTask(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	task, err := s.store.GetTask(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "任务不存在")
		return
	}
	normalizeTask(task)
	ok(c, gin.H{"task": task, "running": s.engine.Running(id)})
}

func (s *Server) cancelTask(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	task, err := s.store.GetTask(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusNotFound, "任务不存在")
		return
	}
	switch task.Status {
	case model.TaskRunning:
		if !s.engine.Cancel(id) {
			s.store.UpdateTask(c.Request.Context(), id, bson.M{
				"status": model.TaskCanceled, "phase": "canceled", "finished_at": time.Now(),
			})
		}
	case model.TaskPending:
		s.store.UpdateTask(c.Request.Context(), id, bson.M{
			"status": model.TaskCanceled, "phase": "canceled", "finished_at": time.Now(),
		})
	default:
		fail(c, http.StatusBadRequest, "任务已结束，无法取消")
		return
	}
	ok(c, gin.H{"message": "任务已取消"})
}

func (s *Server) deleteTask(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	if s.engine.Running(id) {
		fail(c, http.StatusConflict, "任务正在运行，请先取消")
		return
	}
	if err := s.store.DeleteTask(c.Request.Context(), id); err != nil {
		fail(c, http.StatusInternalServerError, "删除任务失败")
		return
	}
	ok(c, gin.H{"message": "已删除"})
}

func (s *Server) taskLogs(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	after, _ := strconv.ParseInt(c.DefaultQuery("after", "0"), 10, 64)
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "300"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 300
	}
	logs, err := s.store.TaskLogsAfter(c.Request.Context(), id, after, int64(limit))
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取日志失败")
		return
	}
	// 增量拉取没有新日志时 Go 的 nil 切片会被编码成 null，前端按数组处理就会报
	// "Cannot read properties of null"；这里统一成空数组。
	if logs == nil {
		logs = []model.TaskLog{}
	}
	last := after
	for _, l := range logs {
		if l.Seq > last {
			last = l.Seq
		}
	}
	ok(c, gin.H{"logs": logs, "last": last})
}

// clearableStatuses 是可以一键清空的任务状态。
var clearableStatuses = []string{model.TaskDone, model.TaskFailed, model.TaskCanceled, model.TaskInterrupted}

// deleteFinishedTasks 批量删除已结束的任务及其日志。
func (s *Server) deleteFinishedTasks(c *gin.Context) {
	statuses := clearableStatuses
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		statuses = nil
		for _, item := range strings.Split(raw, ",") {
			value := strings.TrimSpace(item)
			if value == "" {
				continue
			}
			if !isClearableStatus(value) {
				fail(c, http.StatusBadRequest, "只能删除已完成、失败、已取消或已中断的任务")
				return
			}
			statuses = append(statuses, value)
		}
		if len(statuses) == 0 {
			fail(c, http.StatusBadRequest, "未指定要删除的任务状态")
			return
		}
	}

	n, err := s.store.DeleteFinishedTasks(c.Request.Context(), statuses)
	if err != nil {
		failf(c, http.StatusInternalServerError, "清理任务失败：%s", i18n.Message(lang(c), err))
		return
	}
	ok(c, gin.H{"deleted": n})
}

func isClearableStatus(status string) bool {
	for _, item := range clearableStatuses {
		if item == status {
			return true
		}
	}
	return false
}

// defaultPorts 取设置里的默认端口，未配置时回退到内置列表。
func defaultPorts(ctx context.Context, s *Server) string {
	const fallback = "80,443,8080,8443"
	st, err := s.store.GetScanSettings(ctx)
	if err != nil || strings.TrimSpace(st.DefaultPorts) == "" {
		return fallback
	}
	return st.DefaultPorts
}
