package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/expvintl/scopex/internal/api"
	"github.com/expvintl/scopex/internal/config"
	"github.com/expvintl/scopex/internal/geo"
	"github.com/expvintl/scopex/internal/job"
	"github.com/expvintl/scopex/internal/scanner"
	"github.com/expvintl/scopex/internal/store"
	"github.com/expvintl/scopex/internal/version"
	"github.com/expvintl/scopex/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	logger.Info("scopex 启动", "version", version.Version)

	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		logger.Error("连接 MongoDB 失败", "uri", cfg.MongoURI, "err", err)
		os.Exit(1)
	}
	defer st.Close(context.Background())

	if pass, created, err := st.EnsureAdmin(ctx, cfg.AdminUser, cfg.AdminPass); err != nil {
		logger.Error("初始化管理员失败", "err", err)
	} else if created {
		logger.Warn("已创建初始管理员，请尽快修改密码", "user", cfg.AdminUser, "password", pass)
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
	go func() {
		logger.Info("scopex 启动", "listen", cfg.Listen, "db", cfg.MongoDB, "dev", cfg.DevMode)
		for _, t := range reg.Statuses() {
			logger.Info("扫描工具", "name", t.Name, "available", t.Available, "version", t.Version, "reason", t.Reason)
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP 服务异常", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("收到退出信号，正在关闭")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	engine.Stop()
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
