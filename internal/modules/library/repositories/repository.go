package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

// ErrVersionConflict means the row exists but its version moved on.
var ErrVersionConflict = errors.New("library item version conflict")

type ListFilter struct {
	Kind     string
	Query    string
	Limit    int
	CursorAt *time.Time
	CursorID *uuid.UUID
}

type LibraryRepository interface {
	List(ctx context.Context, tripID uuid.UUID, filter ListFilter) ([]entities.LibraryItem, error)
	Create(ctx context.Context, item entities.LibraryItem) (*entities.LibraryItem, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entities.LibraryItem, error)
	FindByIDs(ctx context.Context, tripID uuid.UUID, ids []uuid.UUID) ([]entities.LibraryItem, error)
	Update(ctx context.Context, id uuid.UUID, version int, data map[string]any) (*entities.LibraryItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// BlocksOfItem returns the blocks that reference the item, with item summary.
	BlocksOfItem(ctx context.Context, itemID uuid.UUID) ([]entities.ScheduleBlock, error)
	DeleteBlocksOfItem(ctx context.Context, itemID uuid.UUID) error
}
