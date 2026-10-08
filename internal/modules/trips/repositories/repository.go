package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
)

type TripListRow struct {
	entities.Trip
	Role        string `db:"role"`
	MemberCount int    `db:"member_count"`
}

type TripRepository interface {
	Create(ctx context.Context, trip entities.Trip) (*entities.Trip, error)
	AddMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]TripListRow, error)
	Find(ctx context.Context, id uuid.UUID) (*entities.Trip, error)
	// Update applies a column map; dates are passed as YYYY-MM-DD text.
	Update(ctx context.Context, id uuid.UUID, data map[string]any) (*entities.Trip, error)
	// Delete removes the trip; its schedule blocks go first so the library
	// RESTRICT constraint cannot trip over the cascade order.
	Delete(ctx context.Context, id uuid.UUID) error
	ListMembers(ctx context.Context, tripID uuid.UUID) ([]entities.TripMember, error)
	FindMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (*entities.TripMember, error)
	// LockOwners locks the owner rows so concurrent demotions cannot orphan a trip.
	LockOwners(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error)
	UpdateMemberRole(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) error
	BlocksOutside(ctx context.Context, tripID uuid.UUID, from time.Time, to time.Time) ([]uuid.UUID, error)
	SyncStatus(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (dto.CalendarSyncStatus, error)
}
