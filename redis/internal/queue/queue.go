package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"time"

	"task-queue/internal/config"
	"task-queue/internal/metrics"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ITaskQueue интерфейс для работы с очередью задач
type ITaskQueue interface {
	AddTask(ctx context.Context, payload string, priority int, executeAt time.Time) error
	ProcessTasks(ctx context.Context)
}

// TaskQueue реализует очередь задач
type TaskQueue struct {
	client        *redis.Client
	metrics       *metrics.Metrics
	cfg           *config.Config
	addTaskScript *redis.Script
	logger        *zap.Logger
}

// NewTaskQueue создаёт новый экземпляр TaskQueue
func NewTaskQueue(client *redis.Client, metrics *metrics.Metrics, cfg *config.Config, logger *zap.Logger) *TaskQueue {
	// Загружаем Lua-скрипт
	scriptPath := filepath.Join("internal", "queue", "scripts", "add_task.lua")
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		logger.Fatal("Failed to load Lua script", zap.Error(err))
	}
	addTaskScript := redis.NewScript(string(scriptContent))

	return &TaskQueue{
		client:        client,
		metrics:       metrics,
		cfg:           cfg,
		addTaskScript: addTaskScript,
		logger:        logger,
	}
}

// AddTask добавляет задачу в очередь с использованием Lua-скрипта
func (tq *TaskQueue) AddTask(ctx context.Context, payload string, priority int, executeAt time.Time) error {
	task := Task{
		ID:        uuid.New().String(),
		Payload:   payload,
		Priority:  priority,
		ExecuteAt: executeAt,
		Attempts:  0,
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		tq.logger.Error("Failed to marshal task",
			zap.String("task_id", task.ID),
			zap.Error(err))
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Выбираем шард на основе хэша task.ID
	shard := tq.getShard(task.ID)

	// Формируем ключи для шарда
	priorityQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.PriorityKey, shard)
	delayedQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.DelayedKey, shard)

	// Логируем входные параметры
	tq.logger.Debug("Executing add_task script",
		zap.String("task_id", task.ID),
		zap.Int("shard", shard),
		zap.Int("priority", priority),
		zap.Int64("execute_at_unix", executeAt.Unix()))

	// Используем Lua-скрипт для атомарного добавления
	result, err := tq.addTaskScript.Run(ctx, tq.client, []string{priorityQueueKey, delayedQueueKey}, taskJSON, priority, executeAt.Unix()).Result()
	if err != nil {
		tq.logger.Error("Failed to execute add_task script",
			zap.String("task_id", task.ID),
			zap.Int("shard", shard),
			zap.Any("result", result),
			zap.Error(err))
		return fmt.Errorf("failed to execute add_task script: %w", err)
	}

	tq.logger.Info("Task added to queue",
		zap.String("task_id", task.ID),
		zap.Int("shard", shard),
		zap.Int("priority", priority))

	return nil
}

// getShard возвращает номер шарда на основе taskID
func (tq *TaskQueue) getShard(taskID string) int {
	hash := crc32.ChecksumIEEE([]byte(taskID))
	return int(hash % uint32(tq.cfg.Queues.Shards))
}
