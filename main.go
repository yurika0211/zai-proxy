package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zai-proxy/internal/config"
	"zai-proxy/internal/handler"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/version"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the YAML config file")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		panic(err)
	}
	logger.InitLogger(config.Cfg.LogLevel)
	version.StartVersionUpdater()

	srv := &http.Server{
		Addr:              config.Cfg.Listen,
		Handler:           handler.NewRouter(),
		ReadHeaderTimeout: time.Duration(config.Cfg.ReadHeaderTimeoutSec) * time.Second,
		ReadTimeout:       time.Duration(config.Cfg.ReadTimeoutSec) * time.Second,
		WriteTimeout:      time.Duration(config.Cfg.WriteTimeoutSec) * time.Second,
		IdleTimeout:       time.Duration(config.Cfg.IdleTimeoutSec) * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		logger.LogInfo("Server starting on %s", config.Cfg.Listen)
		serverErrCh <- srv.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrCh:
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		logger.LogError("Server failed: %v", err)
	case sig := <-stopCh:
		logger.LogInfo("Received signal %s, shutting down", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Cfg.ShutdownTimeoutSec)*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.LogError("Graceful shutdown failed: %v", err)
		}
	}
}
