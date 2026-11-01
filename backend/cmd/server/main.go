package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/expvintl/scopex/internal/api"
	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/geo"
	"github.com/expvintl/scopex/internal/job"
	"github.com/expvintl/scopex/internal/logging"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/store"
	"github.com/expvintl/scopex/internal/version"
	"github.com/expvintl/scopex/web"
)

func main() {
	cfg := config.Load()

	// 屏幕只留关键信息，明细（含每次 HTTP 请求）写日志文件。
	stopLog, err := logging.Setup(logging.Options{
		ConsoleLevel: cfg.Log.ConsoleLevel,
		File:         cfg.Log.File,
		FileLevel:    cfg.Log.Level,
		MaxMB:        cfg.Log.MaxMB,
		Backups:      cfg.Log.Backups,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = stopLog() }()

	logger := slog.Default()
	logPath := logging.FilePath()
	if logPath == "" {
		logPath = "（仅屏幕）"
	}
	logging.Status("scopex %s 正在启动", version.Version)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		logger.Error("连接 MongoDB 失败", "uri", cfg.MongoURI, "err", err)
		os.Exit(1)
	}
	defer st.Close(context.Background())

	// 签名密钥：环境变量优先，否则生成随机密钥保存在数据库里。
	// 数据库被清空后密钥随之更换，历史令牌全部失效。
	secret, secretCreated, err := st.LoadOrCreateAuthSecret(ctx, cfg.JWTSecret)
	if err != nil {
		logger.Error("初始化签名密钥失败", "err", err)
		os.Exit(1)
	}
	cfg.JWTSecret = secret
	if secretCreated {
		logging.Status("未设置 SCOPEX_JWT_SECRET，已自动生成并保存到数据库（数据库重置后所有登录会话失效）")
	}

	if pass, created, err := st.EnsureAdmin(ctx, cfg.AdminUser, cfg.AdminPass); err != nil {
		logger.Error("初始化管理员失败", "err", err)
	} else if created && pass != "" {
		// 首次安装自动生成的密码只出现这一次，必须让用户不可能错过。
		logging.Notice("首次安装：已创建管理员账号",
			[2]string{"用户名", cfg.AdminUser},
			[2]string{"密码", pass},
			[2]string{"提示", "登录后请立即修改密码"},
		)
	} else if created {
		logging.Status("已创建管理员账号 %s，密码来自 SCOPEX_ADMIN_PASS", cfg.AdminUser)
	}
	if n, err := st.BackfillSessionNonces(ctx); err != nil {
		logger.Error("初始化账号会话标识失败", "err", err)
	} else if n > 0 {
		logger.Warn("已为历史账号生成会话标识，此前签发的令牌全部失效", "count", n)
	}
	if n, err := st.RecoverTasks(ctx); err == nil && n > 0 {
		logger.Warn("检测到上次未完成的任务，已标记为中断", "count", n)
	}

	g := geo.New(cfg.Geo, logger)
	if gs, found, err := st.GetGeoSettings(ctx); err != nil {
		logger.Warn("读取地理库设置失败，使用默认值", "err", err)
	} else if found {
		g.Configure(gs.Enabled, gs.AutoDownload, gs.CityURL, gs.ASNURL)
	}
	g.Start(ctx)
	go backfillGeo(ctx, st, g, logger)
	reg := scanner.NewRegistry(cfg.Scanner)
	settings, settingsErr := st.GetScanSettings(ctx)
	if settingsErr != nil {
		logger.Warn("读取扫描设置失败，使用默认值", "err", settingsErr)
	} else {
		reg.ApplySettings(settings)
		if settings.ScanInterface != "" {
			logger.Info("扫描出口网卡", "interface", settings.ScanInterface)
		}
	}
	engine := job.NewEngine(cfg, st, reg, g, logger)
	deny := settings.DenyCIDRs
	if deny == nil {
		deny = cfg.Scanner.DenyCIDRs
	}
	if err := engine.SetDenyCIDRs(deny); err != nil {
		logger.Warn("禁止扫描网段无效，已忽略", "err", err)
	}
	engine.Start(ctx, 2)

	webFS := mustWeb()
	srv := api.New(cfg, st, engine, reg, g, logger, webFS)

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	logging.Status("监听 %s · 数据库 %s · 日志 %s", cfg.Listen, cfg.MongoDB, logPath)
	logTools(reg.Statuses(), logger)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP 服务异常", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logging.Status("收到退出信号，正在关闭")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	engine.Stop()
	logging.Status("已退出")
}

// logTools 把扫描工具明细写进日志文件，屏幕上只保留一行概要。
func logTools(sts []scanner.ToolStatus, logger *slog.Logger) {
	if len(sts) == 0 {
		return
	}
	summary := make([]string, 0, len(sts))
	for _, t := range sts {
		logger.Info("扫描工具", "name", t.Name, "available", t.Available, "version", t.Version, "reason", t.Reason)
		mark := "✗"
		if t.Available {
			mark = "✓"
		}
		summary = append(summary, t.Name+" "+mark)
	}
	logging.Status("扫描工具 %s", strings.Join(summary, " · "))
}

func mustWeb() web.FS {
	fsys, err := web.Dist()
	if err != nil {
		slog.Warn("前端资源未嵌入，仅提供 API", "err", err)
		return nil
	}
	return fsys
}

// backfillGeo 为历史资产补全缺失的归属地信息，库未就绪时自动放弃。
func backfillGeo(ctx context.Context, st *store.Store, g *geo.Resolver, log *slog.Logger) {
	if !g.Enabled() {
		return
	}
	for round := 0; round < 200 && ctx.Err() == nil; round++ {
		time.Sleep(1500 * time.Millisecond)
		ips, err := st.HostsMissingGeo(ctx, 200)
		if err != nil || len(ips) == 0 {
			return
		}
		updated := 0
		for _, ip := range ips {
			loc, asn := g.Lookup(ctx, ip)
			if loc.CountryCode == "" && asn == nil {
				continue
			}
			if n, err := st.SetHostGeo(ctx, ip, loc, asn); err == nil && n > 0 {
				updated++
			}
		}
		if updated == 0 {
			return
		}
		log.Info("补全资产归属地", "ips", updated)
	}
}
