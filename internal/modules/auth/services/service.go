package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/auth/dto"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
)

type LoginStart struct {
	RedirectURL string
	StateCookie string
}

type LoginResult struct {
	Token   string
	Expires time.Time
	Next    string
	User    *dto.UserResponse
}

type AuthService interface {
	StartLogin(ctx context.Context, next string) (*LoginStart, error)
	FinishLogin(ctx context.Context, code string, state string, stateCookie string) (*LoginResult, error)
	DevLogin(ctx context.Context, request dto.DevLoginRequest) (*LoginResult, error)
	// Resolve is used by the auth middleware; it also slides the session expiry.
	Resolve(ctx context.Context, token string) (*requestctx.User, *time.Time, error)
	Logout(ctx context.Context, token string) error
	Me(ctx context.Context, userID uuid.UUID) (*dto.MeResponse, error)
	PurgeExpired(ctx context.Context) (int64, error)
}
