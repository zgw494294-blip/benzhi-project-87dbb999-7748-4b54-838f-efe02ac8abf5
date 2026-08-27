package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cave-sampling-permit/internal/application"
	"cave-sampling-permit/internal/httpapi"
	"cave-sampling-permit/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	repo, err := storage.Open(databasePathForRun(cfg))
	if err != nil {
		return fmt.Errorf("初始化 SQLite: %w", err)
	}
	defer repo.Close()
	ids := &application.RandomIDGenerator{}
	service := application.NewService(repo, application.RealClock{}, ids)
	api := httpapi.New(service, slog.Default())
	server := httpapi.NewServer(api.Handler(), cfg.ShutdownTimeout)
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Address, err)
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	if cfg.Selfcheck {
		return runBoundedSelfcheck(server, listener, serveErrors)
	}
	slog.Info("采样许可服务已启动", "address", listener.Addr().String(), "database", cfg.DatabasePath)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		if err := server.Shutdown(); err != nil {
			return fmt.Errorf("关闭 HTTP 服务: %w", err)
		}
		err := <-serveErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
