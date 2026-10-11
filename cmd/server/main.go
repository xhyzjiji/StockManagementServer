// 造物仓服务端入口：加载配置、迁移数据库、启动 HTTP 服务。
// Entry point of the warehouse server: loads config, migrates the database and starts HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"private/construction_warehouse/internal/auth"
	"private/construction_warehouse/internal/config"
	"private/construction_warehouse/internal/db"
	"private/construction_warehouse/internal/imagestore"
	"private/construction_warehouse/internal/server"
	"private/construction_warehouse/internal/store"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to the YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	conn, err := db.Open(cfg.MySQL)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, conn); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	images, err := imagestore.New(cfg.Storage.UploadDir)
	if err != nil {
		log.Fatalf("init upload dir: %v", err)
	}
	st := store.New(conn, images)
	au := auth.New(st, cfg.Auth)
	if err := au.EnsureInitAdmin(ctx); err != nil {
		log.Fatalf("init admin: %v", err)
	}
	srv := server.New(cfg, st, au)

	go janitor(ctx, st, srv)

	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s (token ttl %s, uploads %s)", cfg.Server.Addr, cfg.Auth.TokenTTL.Duration, images.Root)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http: %v", err)
	}
}

// janitor 定期清理过期 token、超过一天的上传暂存文件与登录限流记录。
// janitor periodically purges expired tokens, staging uploads older than a day and login limiter entries.
func janitor(ctx context.Context, st *store.Store, srv *server.Server) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if err := st.PurgeExpiredTokens(ctx); err != nil && ctx.Err() == nil {
			log.Printf("purge tokens: %v", err)
		}
		st.Images().CleanTmp(24 * time.Hour)
		srv.SweepLimiter()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
