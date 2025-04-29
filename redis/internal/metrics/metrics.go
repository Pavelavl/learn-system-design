package metrics

import (
	"context"
	"strconv"

	"task-queue/internal/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Metrics управляет метриками выполнения задач
type Metrics struct {
	client     *redis.Client
	metricsKey string
	logger     *zap.Logger
}

// NewMetrics создаёт новый экземпляр Metrics
func NewMetrics(client *redis.Client, cfg *config.Config, logger *zap.Logger) *Metrics {
	return &Metrics{
		client:     client,
		metricsKey: cfg.Metrics.Key,
		logger:     logger,
	}
}

// IncrementSuccess увеличивает счётчик успешных задач
func (m *Metrics) IncrementSuccess(ctx context.Context) {
	m.client.HIncrBy(ctx, m.metricsKey, "success", 1)
	m.logger.Debug("Incremented success metric")
}

// IncrementFailed увеличивает счётчик проваленных задач
func (m *Metrics) IncrementFailed(ctx context.Context) {
	m.client.HIncrBy(ctx, m.metricsKey, "failed", 1)
	m.logger.Debug("Incremented failed metric")
}

// IncrementTotalProcessed увеличивает счётчик обработанных задач
func (m *Metrics) IncrementTotalProcessed(ctx context.Context) {
	m.client.HIncrBy(ctx, m.metricsKey, "total_processed", 1)
	m.logger.Debug("Incremented total_processed metric")
}

// IncrementDeadLetter увеличивает счётчик задач в dead_letter_queue
func (m *Metrics) IncrementDeadLetter(ctx context.Context) {
	m.client.HIncrBy(ctx, m.metricsKey, "dead_letter", 1)
	m.logger.Debug("Incremented dead_letter metric")
}

// GetMetrics возвращает текущие метрики
func (m *Metrics) GetMetrics(ctx context.Context) (map[string]int64, error) {
	metrics, err := m.client.HGetAll(ctx, m.metricsKey).Result()
	if err != nil {
		m.logger.Error("Failed to get metrics",
			zap.Error(err))
		return nil, err
	}

	result := make(map[string]int64)
	for k, v := range metrics {
		val, _ := strconv.ParseInt(v, 10, 64)
		result[k] = val
	}

	m.logger.Debug("Retrieved metrics",
		zap.Any("metrics", result))
	return result, nil
}
