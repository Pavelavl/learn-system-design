package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NewClient создаёт новый Redis-клиент
func NewClient(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Проверяем соединение
	if err := client.Ping(ctx).Err(); err != nil {
		zap.L().Error("Failed to ping Redis",
			zap.String("addr", addr),
			zap.Error(err))
		return nil, err
	}

	zap.L().Info("Connected to Redis",
		zap.String("addr", addr))
	return client, nil
}
