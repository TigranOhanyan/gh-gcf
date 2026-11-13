package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey struct{}

func init() {
	logger := MustCreateZuluTimeLogger()
	zap.ReplaceGlobals(logger)
}

func MustCreateZuluTimeLogger() (logger *zap.Logger) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	config.EncoderConfig.EncodeTime = timeEncoder

	var err error
	logger, err = config.Build()
	if err != nil {
		panic(err)
	}
	return
}

func With(ctx context.Context, fields ...zap.Field) (next context.Context) {
	logger := From(ctx).With(fields...)
	next = context.WithValue(ctx, contextKey{}, logger)
	return
}

func From(ctx context.Context) (logger *zap.Logger) {
	logger = zap.L()
	missing := ctx == nil
	if missing {
		return
	}
	value := ctx.Value(contextKey{})
	stored, ok := value.(*zap.Logger)
	present := ok && stored != nil
	if present {
		logger = stored
	}
	return
}
