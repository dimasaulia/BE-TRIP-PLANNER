package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/invites/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/invites/repositories"
	tripDto "github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/crypto"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

const (
	defaultExpiryDays = 7
	maxExpiryDays     = 90
	maxUsesLimit      = 1000
)

type InviteServiceImpl struct {
	repo   repositories.InviteRepository
	db     *database.Database
	access access.Service
	hub    realtime.Publisher
	cfg    config.Config
	log    *logger.LayerLogger
}

func NewInviteService(
	repo repositories.InviteRepository,
	db *database.Database,
	accessService access.Service,
	hub realtime.Publisher,
	cfg config.Config,
	appLogger *logger.Logger,
) InviteService {
	return &InviteServiceImpl{
		repo:   repo,
		db:     db,
		access: accessService,
		hub:    hub,
		cfg:    cfg,
		log:    appLogger.Layer("service.invites"),
	}
}

func (s *InviteServiceImpl) Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateInviteRequest) (*dto.CreateInviteResponse, error) {
	end := s.log.Start(ctx, "Create", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	days := defaultExpiryDays
	if request.ExpiresInDays != nil {
		days = *request.ExpiresInDays
	}

	validator := validation.New()
	validator.Check(request.Role == string(access.Editor) || request.Role == string(access.Viewer), "role", "must be editor or viewer")
	validator.Check(days >= 1 && days <= maxExpiryDays, "expires_in_days", "must be between 1 and 90")
	if request.MaxUses != nil {
		validator.Check(*request.MaxUses >= 1 && *request.MaxUses <= maxUsesLimit, "max_uses", "must be between 1 and 1000")
	}
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	token, err := crypto.RandomToken(32)
	if err != nil {
		end(err)
		return nil, err
	}

	invite, err := s.repo.Create(ctx, tripID, crypto.HashToken(token), request.Role, userID,
		time.Now().Add(time.Duration(days)*24*time.Hour), request.MaxUses)
	if err != nil {
		end(err)
		return nil, err
	}

	end(nil, "invite_id", invite.ID)
	return &dto.CreateInviteResponse{
		ID:        invite.ID,
		TripID:    invite.TripID,
		Role:      invite.Role,
		ExpiresAt: invite.ExpiresAt,
		MaxUses:   invite.MaxUses,
		UseCount:  invite.UseCount,
		Token:     token,
		URL:       s.cfg.App.BaseURL + "/join/" + token,
	}, nil
}

func (s *InviteServiceImpl) List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) ([]dto.InviteItem, error) {
	end := s.log.Start(ctx, "List", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	invites, err := s.repo.ListActive(ctx, tripID)
	if err != nil {
		end(err)
		return nil, err
	}

	items := make([]dto.InviteItem, 0, len(invites))
	for _, invite := range invites {
		items = append(items, dto.InviteItem{
			ID:            invite.ID,
			Role:          invite.Role,
			CreatedBy:     invite.CreatedBy,
			CreatedByName: invite.CreatedByName,
			ExpiresAt:     invite.ExpiresAt,
			MaxUses:       invite.MaxUses,
			UseCount:      invite.UseCount,
			CreatedAt:     invite.CreatedAt,
		})
	}

	end(nil, "count", len(items))
	return items, nil
}

func (s *InviteServiceImpl) Revoke(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, inviteID uuid.UUID) error {
	end := s.log.Start(ctx, "Revoke", "trip_id", tripID, "invite_id", inviteID)

	membership, err := s.access.Require(ctx, tripID, userID, access.Editor)
	if err != nil {
		end(err)
		return err
	}

	invite, err := s.repo.FindByID(ctx, tripID, inviteID)
	if err != nil {
		end(err)
		return err
	}

	// Owners may revoke anything, editors only what they created themselves.
	if membership.Role != access.Owner && invite.CreatedBy != userID {
		err := apperror.Forbidden("forbidden").With("required_role", string(access.Owner))
		end(err)
		return err
	}

	err = s.repo.Revoke(ctx, inviteID)
	end(err)
	return err
}

// resolve finds a usable invite and tells expired, exhausted and revoked apart.
func (s *InviteServiceImpl) resolve(ctx context.Context, token string, lock bool) (*entities.TripInvite, error) {
	invite, err := s.repo.FindByTokenHash(ctx, crypto.HashToken(token), lock)
	if err != nil {
		return nil, err
	}

	if invite.RevokedAt != nil {
		return nil, apperror.NotFound("invite_not_found")
	}
	if !invite.ExpiresAt.After(time.Now()) {
		return nil, apperror.Unprocessable("invite_expired")
	}
	return invite, nil
}

func (s *InviteServiceImpl) Preview(ctx context.Context, token string) (*dto.PreviewResponse, error) {
	end := s.log.Start(ctx, "Preview")

	invite, err := s.resolve(ctx, token, false)
	if err != nil {
		end(err)
		return nil, err
	}
	if invite.MaxUses != nil && invite.UseCount >= *invite.MaxUses {
		err := apperror.Unprocessable("invite_exhausted")
		end(err)
		return nil, err
	}

	end(nil)
	return &dto.PreviewResponse{TripName: invite.TripName, Role: invite.Role, ExpiresAt: invite.ExpiresAt}, nil
}

// Accept is idempotent: an existing member keeps their role and no use is spent.
func (s *InviteServiceImpl) Accept(ctx context.Context, userID uuid.UUID, token string) (*dto.AcceptResponse, error) {
	end := s.log.Start(ctx, "Accept")

	var (
		result *dto.AcceptResponse
		added  *entities.TripMember
	)

	err := s.db.InTx(ctx, func(ctx context.Context) error {
		invite, err := s.resolve(ctx, token, true)
		if err != nil {
			return err
		}

		role, isMember, err := s.repo.MemberRole(ctx, invite.TripID, userID)
		if err != nil {
			return err
		}
		if isMember {
			result = &dto.AcceptResponse{TripID: invite.TripID, Role: role, Joined: false}
			return nil
		}

		if invite.MaxUses != nil && invite.UseCount >= *invite.MaxUses {
			return apperror.Unprocessable("invite_exhausted")
		}

		added, err = s.repo.AddMember(ctx, invite.TripID, userID, invite.Role)
		if err != nil {
			return err
		}
		if err := s.repo.IncrementUse(ctx, invite.ID); err != nil {
			return err
		}

		result = &dto.AcceptResponse{TripID: invite.TripID, Role: invite.Role, Joined: true}
		return nil
	})
	if err != nil {
		end(err)
		return nil, err
	}

	if added != nil {
		s.hub.Publish(added.TripID, "member.changed", requestctx.ClientID(ctx), tripDto.MemberChange{
			Action: "added", UserID: userID, Member: added,
		})
	}

	end(nil, "joined", result.Joined)
	return result, nil
}
