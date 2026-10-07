package services

import (
	"context"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/dto"
)

type ReleaseNoteService interface {
	Find(ctx context.Context, limit uint64, offset uint64) ([]entities.ReleaseNote, error)
	FindByID(ctx context.Context, id int64) (*entities.ReleaseNote, error)
	Create(ctx context.Context, request dto.CreateReleaseNoteRequest) (*entities.ReleaseNote, error)
	Update(ctx context.Context, id int64, request dto.UpdateReleaseNoteRequest) (*entities.ReleaseNote, error)
	Delete(ctx context.Context, id int64, deletedBy *int64) error
}
