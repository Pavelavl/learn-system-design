package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ProcessTasks запускает воркер для обработки задач
func (tq *TaskQueue) ProcessTasks(ctx context.Context) {
	for shard := 0; shard < tq.cfg.Queues.Shards; shard++ {
		go tq.processShard(ctx, shard)
	}
}

// processShard обрабатывает задачи для одного шарда
func (tq *TaskQueue) processShard(ctx context.Context, shard int) {
	priorityQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.PriorityKey, shard)
	delayedQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.DelayedKey, shard)
	processingQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.ProcessingKey, shard)

	go tq.processDelayedTasks(ctx, shard) // Запускаем обработку отложенных задач

	for {
		select {
		case <-ctx.Done():
			tq.logger.Info("Stopping task processing for shard due to context cancellation",
				zap.Int("shard", shard))
			return
		default:
			// Извлекаем задачу с наивысшим приоритетом
			result, err := tq.client.BZPopMax(ctx, 0, priorityQueueKey).Result()
			if err != nil {
				tq.logger.Error("Error popping task from shard",
					zap.Int("shard", shard),
					zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			// Переносим задачу в processing_queue
			taskJSON := result.Member.(string)
			err = tq.client.LPush(ctx, processingQueueKey, taskJSON).Err()
			if err != nil {
				tq.logger.Error("Error moving task to processing queue",
					zap.Int("shard", shard),
					zap.Error(err))
				continue
			}

			// Десериализуем задачу
			var task Task
			if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
				tq.logger.Error("Error unmarshaling task",
					zap.Int("shard", shard),
					zap.Error(err))
				tq.client.LRem(ctx, processingQueueKey, 1, taskJSON)
				continue
			}

			// Обрабатываем задачу
			err = tq.processTask(task)
			if err != nil {
				tq.logger.Error("Error processing task",
					zap.String("task_id", task.ID),
					zap.Int("shard", shard),
					zap.Int("attempt", task.Attempts+1),
					zap.Error(err))
				task.Attempts++
				if task.Attempts >= tq.cfg.Retry.MaxAttempts {
					// Перемещаем в dead_letter_queue
					tq.client.LPush(ctx, "dead_letter_queue", taskJSON)
					tq.logger.Warn("Task moved to dead_letter_queue after max attempts",
						zap.String("task_id", task.ID),
						zap.Int("attempts", task.Attempts))
					tq.metrics.IncrementDeadLetter(ctx)
				} else {
					// Вычисляем задержку с экспоненциальным backoff
					delay := time.Duration(tq.cfg.Retry.BackoffInitial) * time.Millisecond
					delay = delay * time.Duration(math.Pow(float64(tq.cfg.Retry.BackoffFactor), float64(task.Attempts-1)))
					task.ExecuteAt = time.Now().Add(delay)
					taskJSONbb, _ := json.Marshal(task)
					taskJSON = string(taskJSONbb)
					// Возвращаем задачу в delayed_queue
					tq.client.ZAdd(ctx, delayedQueueKey, redis.Z{
						Score:  float64(task.ExecuteAt.Unix()),
						Member: taskJSON,
					})
					tq.logger.Info("Task scheduled for retry",
						zap.String("task_id", task.ID),
						zap.Duration("delay", delay),
						zap.Int("attempt", task.Attempts))
				}
			} else {
				tq.logger.Info("Task processed successfully",
					zap.String("task_id", task.ID),
					zap.Int("shard", shard))
				tq.metrics.IncrementSuccess(ctx)
			}

			// Удаляем задачу из processing_queue
			tq.client.LRem(ctx, processingQueueKey, 1, taskJSON)
			tq.metrics.IncrementTotalProcessed(ctx)
		}
	}
}

// processTask выполняет задачу (здесь заглушка с возможной ошибкой)
func (tq *TaskQueue) processTask(task Task) error {
	tq.logger.Debug("Processing task",
		zap.String("task_id", task.ID),
		zap.String("payload", task.Payload),
		zap.Int("attempt", task.Attempts+1))
	// Имитация обработки
	time.Sleep(100 * time.Millisecond)
	return nil
}

// processDelayedTasks переносит отложенные задачи в priority_queue
func (tq *TaskQueue) processDelayedTasks(ctx context.Context, shard int) {
	delayedQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.DelayedKey, shard)
	priorityQueueKey := fmt.Sprintf("%s:%d", tq.cfg.Queues.PriorityKey, shard)

	for {
		select {
		case <-ctx.Done():
			tq.logger.Info("Stopping delayed task processing for shard due to context cancellation",
				zap.Int("shard", shard))
			return
		default:
			now := time.Now().Unix()
			// Извлекаем задачи, чьё время выполнения наступило
			tasks, err := tq.client.ZRangeByScore(ctx, delayedQueueKey, &redis.ZRangeBy{
				Min:    "-inf",
				Max:    fmt.Sprintf("%d", now),
				Offset: 0,
				Count:  100,
			}).Result()
			if err != nil {
				tq.logger.Error("Error fetching delayed tasks",
					zap.Int("shard", shard),
					zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			for _, taskJSON := range tasks {
				var task Task
				if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
					tq.logger.Error("Error unmarshaling delayed task",
						zap.Int("shard", shard),
						zap.Error(err))
					continue
				}

				// Переносим задачу в priority_queue
				err = tq.client.ZAdd(ctx, priorityQueueKey, redis.Z{
					Score:  float64(task.Priority),
					Member: taskJSON,
				}).Err()
				if err != nil {
					tq.logger.Error("Error moving delayed task to priority queue",
						zap.Int("shard", shard),
						zap.Error(err))
					continue
				}

				// Удаляем задачу из delayed_queue
				tq.client.ZRem(ctx, delayedQueueKey, taskJSON)
				tq.logger.Debug("Moved delayed task to priority queue",
					zap.String("task_id", task.ID),
					zap.Int("shard", shard))
			}

			// Ждём следующую задачу или 1 секунду
			if len(tasks) == 0 {
				time.Sleep(time.Second)
			}
		}
	}
}
