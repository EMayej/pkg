package xlog

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type correlationIDType int

const (
	requestIDKey correlationIDType = iota
)

// Level exposes Log Level of current zap Logger so we can dynamic set log level
// if we expose the handler. For example, http.Handle("/loglvlz", xlog.Level)
var Level zap.AtomicLevel

func init() {
	zapConfig := zap.NewProductionConfig()
	zapConfig.Encoding = "console"
	zapConfig.DisableCaller = true
	zapConfig.DisableStacktrace = true
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	logger, err = zapConfig.Build()
	if err != nil {
		panic("fail to build logger")
	}

	logger = logger.With(
		zap.Int("pid", os.Getpid()),
	)

	Level = zapConfig.Level
}

// WithRequestID returns a context which knows its request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

var logger *zap.Logger

// Logger returns a zap logger with as much context as possible
func Logger(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return logger
	}

	newLogger := logger
	if ctxRequestID, ok := ctx.Value(requestIDKey).(string); ok {
		newLogger = newLogger.With(zap.String("requestID", ctxRequestID))
	}
	return newLogger
}

// Sync flushes logs to disk
func Sync() error {
	return logger.Sync()
}
