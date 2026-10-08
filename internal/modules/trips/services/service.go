package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
)

type TripService interface {
	Create(ctx context.Context, userID uuid.UUID, request dto.CreateTripRequest) (*dto.TripResponse, error)
	List(ctx context.Context, userID uuid.UUID) ([]dto.TripListItem, error)
	Get(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.TripDetailResponse, error)
	Update(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.UpdateTripRequest) (*dto.TripResponse, error)
	Delete(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) error
	ListMembers(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) ([]entities.TripMember, error)
	UpdateMemberRole(ctx context.Context, actorID uuid.UUID, tripID uuid.UUID, targetID uuid.UUID, request dto.UpdateMemberRequest) (*entities.TripMember, error)
	RemoveMember(ctx context.Context, actorID uuid.UUID, tripID uuid.UUID, targetID uuid.UUID) error
}
