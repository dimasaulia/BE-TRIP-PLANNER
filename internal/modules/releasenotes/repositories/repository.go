package repositories

import (
	"context"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

type ReleaseNoteRepository interface {
	Find(ctx context.Context, limit uint64, offset uint64) ([]entities.ReleaseNote, error)
	FindByID(ctx context.Context, id int64) (*entities.ReleaseNote, error)
	Create(ctx context.Context, releaseNote entities.ReleaseNote) (*entities.ReleaseNote, error)
	Update(ctx context.Context, id int64, data map[string]any) (*entities.ReleaseNote, error)
	SoftDelete(ctx context.Context, id int64, deletedBy *int64) error
}
