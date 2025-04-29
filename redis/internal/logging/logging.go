package logging

import (
	"fmt"
	"task-queue/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger создаёт новый Zap-логгер на основе конфигурации
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	var zapCfg zap.Config

	switch cfg.Logging.Format {
	case "json":
		zapCfg = zap.NewProductionConfig()
	case "console":
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default:
		return nil, fmt.Errorf("invalid logging configuration")
	}

	switch cfg.Logging.Level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return nil, fmt.Errorf("invalid logging configuration")
	}

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}
