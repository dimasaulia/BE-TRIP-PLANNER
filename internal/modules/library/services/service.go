package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/library/dto"
)

type LibraryService interface {
	List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, kind string, query string, cursor string, limit int) (*dto.ListResponse, error)
	Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateItemRequest) (*entities.LibraryItem, error)
	Get(ctx context.Context, userID uuid.UUID, itemID uuid.UUID) (*entities.LibraryItem, error)
	Update(ctx context.Context, userID uuid.UUID, itemID uuid.UUID, request dto.UpdateItemRequest) (*entities.LibraryItem, error)
	Delete(ctx context.Context, userID uuid.UUID, itemID uuid.UUID, force bool) error
	Export(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.ExportRequest) (*dto.ExportResponse, error)
}
