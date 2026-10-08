package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/invites/dto"
)

type InviteService interface {
	Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateInviteRequest) (*dto.CreateInviteResponse, error)
	List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) ([]dto.InviteItem, error)
	Revoke(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, inviteID uuid.UUID) error
	Preview(ctx context.Context, token string) (*dto.PreviewResponse, error)
	Accept(ctx context.Context, userID uuid.UUID, token string) (*dto.AcceptResponse, error)
}
