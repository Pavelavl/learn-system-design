package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-queue/internal/api"
	"task-queue/internal/config"
	"task-queue/internal/logging"
	"task-queue/internal/metrics"
	"task-queue/internal/queue"
	"task-queue/internal/redis"

	"go.uber.org/zap"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := logging.NewLogger(cfg)
	if err != nil {
		zap.L().Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	redisClient, err := redis.NewClient(ctx, cfg.Redis.Addr)
	if err != nil {
		logger.Fatal("Failed to initialize Redis client: %v", zap.Error(err))
	}
	defer redisClient.Close()

	metrics := metrics.NewMetrics(redisClient, cfg, logger)

	tq := queue.NewTaskQueue(redisClient, metrics, cfg, logger)

	go tq.ProcessTasks(ctx)

	handler := api.NewHandler(tq, cfg, logger)
	srv := &http.Server{
		Addr:    cfg.HTTP.Port,
		Handler: handler,
	}

	go func() {
		logger.Info("Starting HTTP server", zap.String("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Received shutdown signal, initiating graceful shutdown...")

	cancel()

	time.Sleep(1 * time.Second)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Info("HTTP server shutdown error", zap.Error(err))
	}

	logger.Info("Application stopped")
}
