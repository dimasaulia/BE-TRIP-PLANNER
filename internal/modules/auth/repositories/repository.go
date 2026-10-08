package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

type SessionRecord struct {
	SessionID uuid.UUID
	ExpiresAt time.Time
	User      entities.User
}

type AuthRepository interface {
	UpsertUser(ctx context.Context, googleSub string, email string, name string, avatarURL *string) (*entities.User, error)
	FindUser(ctx context.Context, id uuid.UUID) (*entities.User, error)
	CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	FindSession(ctx context.Context, tokenHash []byte) (*SessionRecord, error)
	ExtendSession(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	FindCalendarConnection(ctx context.Context, userID uuid.UUID) (*entities.CalendarConnection, error)
}
