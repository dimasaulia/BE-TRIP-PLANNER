package requestctx

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	languageKey  contextKey = "language"
	startTimeKey contextKey = "start_time"
	clientIDKey  contextKey = "client_id"
	userKey      contextKey = "user"
)

// User is the authenticated caller resolved from the session cookie.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func UserFrom(ctx context.Context) (User, bool) {
	if ctx == nil {
		return User{}, false
	}

	user, ok := ctx.Value(userKey).(User)
	return user, ok
}

// WithClientID stores the X-Client-Id header so realtime events can name the sender.
func WithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDKey, clientID)
}

func ClientID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	value, _ := ctx.Value(clientIDKey).(string)
	return value
}

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
