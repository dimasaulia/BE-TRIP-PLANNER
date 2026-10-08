package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/dto"
)

type ScheduleService interface {
	List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, from string, to string) (*dto.ListResponse, error)
	Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateBlockRequest) (*entities.BlockView, error)
	Update(ctx context.Context, userID uuid.UUID, blockID uuid.UUID, request dto.UpdateBlockRequest) (*entities.BlockView, error)
	Delete(ctx context.Context, userID uuid.UUID, blockID uuid.UUID) error
}
