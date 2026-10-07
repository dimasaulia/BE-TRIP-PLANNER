package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/platform/sanitizer"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
)

type Config struct {
	Level  string
	LogDir string
}

type Logger struct {
	base        *slog.Logger
	dailyWriter *dailyWriter
}

type LayerLogger struct {
	base  *slog.Logger
	layer string
}

func New(config Config) (*Logger, error) {
	level := parseLevel(config.Level)
	if config.LogDir == "" {
		config.LogDir = "logs"
	}

	fileWriter := newDailyWriter(config.LogDir)
	writer := io.MultiWriter(os.Stdout, fileWriter)

	base := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	}))

	return &Logger{
		base:        base,
		dailyWriter: fileWriter,
	}, nil
}

func (l *Logger) Close() error {
	return l.dailyWriter.Close()
}

func (l *Logger) Layer(layer string) *LayerLogger {
	return &LayerLogger{
		base:  l.base.With("layer", layer),
		layer: layer,
	}
}

func (l *LayerLogger) Start(ctx context.Context, function string, attrs ...any) func(error, ...any) {
	start := time.Now()
	l.Info(ctx, function+".start", attrs...)

	return func(err error, endAttrs ...any) {
		attrs := []any{"duration_ms", time.Since(start).Milliseconds()}
		attrs = append(attrs, endAttrs...)

		if err != nil {
			attrs = append(attrs, "error", sanitizer.Error(err))
			l.base.ErrorContext(ctx, function+".end", l.withContext(ctx, sanitizer.Attrs(attrs))...)
			return
		}

		l.base.InfoContext(ctx, function+".end", l.withContext(ctx, sanitizer.Attrs(attrs))...)
	}
}

func (l *LayerLogger) Info(ctx context.Context, message string, attrs ...any) {
	l.base.InfoContext(ctx, message, l.withContext(ctx, sanitizer.Attrs(attrs))...)
}

func (l *LayerLogger) Warn(ctx context.Context, message string, attrs ...any) {
	l.base.WarnContext(ctx, message, l.withContext(ctx, sanitizer.Attrs(attrs))...)
}

func (l *LayerLogger) Error(ctx context.Context, message string, err error, attrs ...any) {
	if err != nil {
		attrs = append(attrs, "error", sanitizer.Error(err))
	}

	l.base.ErrorContext(ctx, message, l.withContext(ctx, sanitizer.Attrs(attrs))...)
}

func (l *LayerLogger) Debug(ctx context.Context, message string, attrs ...any) {
	l.base.DebugContext(ctx, message, l.withContext(ctx, sanitizer.Attrs(attrs))...)
}

func (l *LayerLogger) withContext(ctx context.Context, attrs []any) []any {
	enriched := []any{
		"request_id", requestctx.RequestID(ctx),
		"language", requestctx.Language(ctx),
	}

	return append(enriched, attrs...)
}

func parseLevel(level string) slog.Leveler {
	switch level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
