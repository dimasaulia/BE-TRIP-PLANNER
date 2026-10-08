package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

type InviteRepository interface {
	Create(ctx context.Context, tripID uuid.UUID, tokenHash []byte, role string, createdBy uuid.UUID, expiresAt time.Time, maxUses *int) (*entities.TripInvite, error)
	ListActive(ctx context.Context, tripID uuid.UUID) ([]entities.TripInvite, error)
	FindByID(ctx context.Context, tripID uuid.UUID, inviteID uuid.UUID) (*entities.TripInvite, error)
	Revoke(ctx context.Context, inviteID uuid.UUID) error
	// FindByTokenHash returns the invite with trip name; lock=true takes a row lock
	// so concurrent accepts cannot exceed max_uses.
	FindByTokenHash(ctx context.Context, tokenHash []byte, lock bool) (*entities.TripInvite, error)
	IncrementUse(ctx context.Context, inviteID uuid.UUID) error
	MemberRole(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (string, bool, error)
	AddMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) (*entities.TripMember, error)
}
