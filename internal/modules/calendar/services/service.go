package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/dto"
)

type ConnectStart struct {
	RedirectURL string
	StateCookie string
}

type CalendarService interface {
	StartConnect(ctx context.Context, userID uuid.UUID, next string) (*ConnectStart, error)
	// FinishConnect stores the refresh token and returns the path to send the user back to.
	FinishConnect(ctx context.Context, userID uuid.UUID, code string, state string, stateCookie string) (string, error)
	Disconnect(ctx context.Context, userID uuid.UUID) error
	EnableSync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.SyncStatusResponse, error)
	DisableSync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, keepEvents bool) error
	Resync(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.SyncStatusResponse, error)
}
