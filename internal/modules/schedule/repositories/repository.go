package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

var (
	// ErrVersionConflict means the row exists but its version moved on.
	ErrVersionConflict = errors.New("schedule block version conflict")
	// ErrOverlap is the exclusion constraint firing, i.e. a raced overlap.
	ErrOverlap = errors.New("schedule block overlaps another block")
)

type ScheduleRepository interface {
	List(ctx context.Context, tripID uuid.UUID, from *time.Time, to *time.Time) ([]entities.ScheduleBlock, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entities.ScheduleBlock, error)
	Create(ctx context.Context, block entities.ScheduleBlock) (uuid.UUID, error)
	Update(ctx context.Context, id uuid.UUID, version int, data map[string]any) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Conflicts lists blocks of the trip overlapping [start, end), ignoring excludeID.
	Conflicts(ctx context.Context, tripID uuid.UUID, start time.Time, end time.Time, excludeID *uuid.UUID) ([]uuid.UUID, error)
	LibraryItemInTrip(ctx context.Context, itemID uuid.UUID, tripID uuid.UUID) (bool, error)
}
