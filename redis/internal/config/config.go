package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config содержит настройки приложения
type Config struct {
	Redis      RedisConfig      `mapstructure:"redis"`
	HTTP       HTTPConfig       `mapstructure:"http"`
	Queues     QueuesConfig     `mapstructure:"queues"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Priorities PrioritiesConfig `mapstructure:"priorities"`
	Retry      RetryConfig      `mapstructure:"retry"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// RedisConfig настройки Redis
type RedisConfig struct {
	Addr string `mapstructure:"addr"`
}

// HTTPConfig настройки HTTP-сервера
type HTTPConfig struct {
	Port string `mapstructure:"port"`
}

// QueuesConfig ключи очередей
type QueuesConfig struct {
	PriorityKey   string `mapstructure:"priority_key"`
	DelayedKey    string `mapstructure:"delayed_key"`
	ProcessingKey string `mapstructure:"processing_key"`
	Shards        int    `mapstructure:"shards"`
}

// MetricsConfig ключ метрик
type MetricsConfig struct {
	Key string `mapstructure:"key"`
}

// PrioritiesConfig приоритеты задач
type PrioritiesConfig struct {
	Low    int `mapstructure:"low"`
	Medium int `mapstructure:"medium"`
	High   int `mapstructure:"high"`
}

// RetryConfig настройки повторов
type RetryConfig struct {
	MaxAttempts    int `mapstructure:"max_attempts"`
	BackoffInitial int `mapstructure:"backoff_initial"`
	BackoffFactor  int `mapstructure:"backoff_factor"`
}

// LoggingConfig настройки логирования
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// LoadConfig загружает конфигурацию из config.yaml
func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
