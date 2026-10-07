package requestctx

import (
	"context"
	"time"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	languageKey  contextKey = "language"
	startTimeKey contextKey = "start_time"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithLanguage(ctx context.Context, language string) context.Context {
	return context.WithValue(ctx, languageKey, language)
}

func Language(ctx context.Context) string {
	if ctx == nil {
		return "id"
	}

	value, _ := ctx.Value(languageKey).(string)
	if value == "" {
		return "id"
	}
	return value
}

func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, startTimeKey, startTime)
}

func StartTime(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Time{}
	}

	value, _ := ctx.Value(startTimeKey).(time.Time)
	return value
}
