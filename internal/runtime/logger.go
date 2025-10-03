package runtime

import (
	"go.uber.org/zap"
)

type HTTPLogger struct {
	*zap.Logger
}

func NewHTTPLogger() *HTTPLogger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	logger, _ := config.Build()

	return &HTTPLogger{Logger: logger}
}

func (logger *HTTPLogger) LogRequest(method, uri string, status, responseSize int, duration float64) {
	logger.Info("HTTP request",
		zap.String("method", method),
		zap.String("uri", uri),
		zap.Int("status", status),
		zap.Int("response_size", responseSize),
		zap.Float64("duration_ms", duration),
	)
}
