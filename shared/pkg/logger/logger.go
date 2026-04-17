// Package logger provides a structured JSON logger based on uber-go/zap.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a production-ready zap logger.
// Set LOG_LEVEL=debug for human-readable output during development.
func New(serviceName string) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = zapcore.DebugLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    encoderCfg,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		InitialFields: map[string]interface{}{
			"service": serviceName,
		},
	}

	return config.Build()
}

// MustNew creates a logger and panics on error. Safe to use in main().
func MustNew(serviceName string) *zap.Logger {
	l, err := New(serviceName)
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	return l
}
