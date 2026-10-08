package services

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/platform/storage"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/attachments"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/syncqueue"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

type TripServiceImpl struct {
	repo   repositories.TripRepository
	db     *database.Database
	access access.Service
	hub    realtime.Publisher
	store  *storage.Store
	queue  syncqueue.Queue
	files  attachments.Service
	log    *logger.LayerLogger
}

func NewTripService(
	repo repositories.TripRepository,
	db *database.Database,
	accessService access.Service,
	hub realtime.Publisher,
	store *storage.Store,
	queue syncqueue.Queue,
	files attachments.Service,
	appLogger *logger.Logger,
) TripService {
	return &TripServiceImpl{
		repo:   repo,
		db:     db,
		access: accessService,
		hub:    hub,
		store:  store,
		queue:  queue,
		files:  files,
		log:    appLogger.Layer("service.trips"),
	}
}

func validateTrip(trip entities.Trip) error {
	validator := validation.New()

	name := strings.TrimSpace(trip.Name)
	validator.Check(name != "", "name", "is required")
	validator.Check(utf8.RuneCountInString(name) <= 120, "name", "must be at most 120 characters")
	validator.Check(utf8.RuneCountInString(trip.Description) <= 5000, "description", "must be at most 5000 characters")

	start, startErr := time.Parse(timeutil.DateLayout, trip.StartDate)
	end, endErr := time.Parse(timeutil.DateLayout, trip.EndDate)
	validator.Check(startErr == nil, "start_date", "must be YYYY-MM-DD")
	validator.Check(endErr == nil, "end_date", "must be YYYY-MM-DD")
	if startErr == nil && endErr == nil {
		validator.Check(!end.Before(start), "end_date", "must not be before start_date")
	}

	_, tzErr := time.LoadLocation(trip.Timezone)
	validator.Check(trip.Timezone != "" && trip.Timezone != "Local" && tzErr == nil, "timezone", "must be an IANA timezone such as Asia/Jakarta")

	return validator.Err()
}

// validateBanner accepts an external http(s) image URL, or an image uploaded to
// this very trip (tripID uuid.Nil means "not created yet": external only).
func (s *TripServiceImpl) validateBanner(ctx context.Context, tripID uuid.UUID, value string) error {
	invalid := validation.New().ErrWith("banner_url", "must be an http(s) URL or an image uploaded to this trip")

	if tripID != uuid.Nil {
		if match := attachments.FilePattern(tripID).FindStringSubmatch(value); match != nil && value == attachments.FileURL(tripID, match[1]) {
			if _, err := s.files.Find(ctx, tripID, match[1]); err != nil {
				return invalid
			}
			return nil
		}
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(value) > 2048 {
		return invalid
	}
	return nil
}

func (s *TripServiceImpl) Create(ctx context.Context, userID uuid.UUID, request dto.CreateTripRequest) (*dto.TripResponse, error) {
	end := s.log.Start(ctx, "Create")

	candidate := entities.Trip{
		Name:        strings.TrimSpace(request.Name),
		Description: request.Description,
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		Timezone:    request.Timezone,
		BannerURL:   request.BannerURL,
		CreatedBy:   userID,
	}
	if err := validateTrip(candidate); err != nil {
		end(err)
		return nil, err
	}
	if candidate.BannerURL != nil {
		if err := s.validateBanner(ctx, uuid.Nil, *candidate.BannerURL); err != nil {
			end(err)
			return nil, err
		}
	}

	var created *entities.Trip
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		trip, err := s.repo.Create(ctx, candidate)
		if err != nil {
			return err
		}
		created = trip
		return s.repo.AddMember(ctx, trip.ID, userID, string(access.Owner))
	})
	if err != nil {
		end(err)
		return nil, err
	}

	end(nil, "trip_id", created.ID)
	return &dto.TripResponse{Trip: *created, MyRole: string(access.Owner)}, nil
}

func (s *TripServiceImpl) List(ctx context.Context, userID uuid.UUID) ([]dto.TripListItem, error) {
	end := s.log.Start(ctx, "List")

	rows, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		end(err)
		return nil, err
	}

	items := make([]dto.TripListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.TripListItem{Trip: row.Trip, MyRole: row.Role, MemberCount: row.MemberCount})
	}

	end(nil, "count", len(items))
	return items, nil
}

func (s *TripServiceImpl) Get(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) (*dto.TripDetailResponse, error) {
	end := s.log.Start(ctx, "Get", "trip_id", tripID)

	membership, err := s.access.Require(ctx, tripID, userID, access.Viewer)
	if err != nil {
		end(err)
		return nil, err
	}

	trip, err := s.repo.Find(ctx, tripID)
	if err != nil {
		end(err)
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, tripID)
	if err != nil {
		end(err)
		return nil, err
	}
	syncStatus, err := s.repo.SyncStatus(ctx, tripID, userID)
	if err != nil {
		end(err)
		return nil, err
	}

	end(nil)
	return &dto.TripDetailResponse{
		Trip:         *trip,
		MyRole:       string(membership.Role),
		Members:      members,
		CalendarSync: syncStatus,
	}, nil
}

func (s *TripServiceImpl) Update(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.UpdateTripRequest) (*dto.TripResponse, error) {
	end := s.log.Start(ctx, "Update", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Owner); err != nil {
		end(err)
		return nil, err
	}

	var updated *entities.Trip
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		current, err := s.repo.Find(ctx, tripID)
		if err != nil {
			return err
		}

		merged := *current
		data := map[string]any{}
		if request.Name != nil {
			merged.Name = strings.TrimSpace(*request.Name)
			data["name"] = merged.Name
		}
		if request.Description != nil {
			merged.Description = *request.Description
			data["description"] = merged.Description
		}
		if request.StartDate != nil {
			merged.StartDate = *request.StartDate
			data["start_date"] = merged.StartDate
		}
		if request.EndDate != nil {
			merged.EndDate = *request.EndDate
			data["end_date"] = merged.EndDate
		}
		if request.Timezone != nil {
			merged.Timezone = *request.Timezone
			data["timezone"] = merged.Timezone
		}
		if request.BannerURL.Set {
			data["banner_set"] = true
			data["banner_url"] = request.BannerURL.Ptr()
			if !request.BannerURL.Null {
				if err := s.validateBanner(ctx, tripID, request.BannerURL.Value); err != nil {
					return err
				}
			}
		}
		if len(data) == 0 {
			return validation.New().ErrWith("body", "at least one field is required")
		}
		if err := validateTrip(merged); err != nil {
			return err
		}

		// Blocks already scheduled must still fit the new dates and timezone.
		rangeChanged := merged.StartDate != current.StartDate || merged.EndDate != current.EndDate || merged.Timezone != current.Timezone
		if rangeChanged {
			from, to, err := timeutil.TripRange(merged.StartDate, merged.EndDate, merged.Location())
			if err != nil {
				return err
			}
			outside, err := s.repo.BlocksOutside(ctx, tripID, from, to)
			if err != nil {
				return err
			}
			if len(outside) > 0 {
				return apperror.Unprocessable("outside_trip_range").With("block_ids", outside)
			}
		}

		updated, err = s.repo.Update(ctx, tripID, data)
		if err != nil {
			return err
		}

		if merged.Timezone != current.Timezone {
			return s.queue.EnqueueTripResync(ctx, tripID)
		}
		return nil
	})
	if err != nil {
		end(err)
		return nil, err
	}

	s.hub.Publish(tripID, "trip.updated", requestctx.ClientID(ctx), updated)

	end(nil)
	return &dto.TripResponse{Trip: *updated, MyRole: string(access.Owner)}, nil
}

func (s *TripServiceImpl) Delete(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) error {
	end := s.log.Start(ctx, "Delete", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Owner); err != nil {
		end(err)
		return err
	}

	err := s.db.InTx(ctx, func(ctx context.Context) error {
		return s.repo.Delete(ctx, tripID)
	})
	if err != nil {
		end(err)
		return err
	}

	s.hub.CloseTrip(tripID)
	if err := s.store.RemoveTrip(tripID); err != nil {
		s.log.Warn(ctx, "files.remove.failed", "trip_id", tripID, "error", err.Error())
	}

	end(nil)
	return nil
}

func (s *TripServiceImpl) ListMembers(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) ([]entities.TripMember, error) {
	end := s.log.Start(ctx, "ListMembers", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return nil, err
	}

	members, err := s.repo.ListMembers(ctx, tripID)
	end(err)
	return members, err
}

func (s *TripServiceImpl) UpdateMemberRole(ctx context.Context, actorID uuid.UUID, tripID uuid.UUID, targetID uuid.UUID, request dto.UpdateMemberRequest) (*entities.TripMember, error) {
	end := s.log.Start(ctx, "UpdateMemberRole", "trip_id", tripID, "target", targetID)

	if _, err := s.access.Require(ctx, tripID, actorID, access.Owner); err != nil {
		end(err)
		return nil, err
	}

	role := access.Role(request.Role)
	validator := validation.New()
	validator.Check(role.Valid(), "role", "must be owner, editor or viewer")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	var member *entities.TripMember
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		owners, err := s.repo.LockOwners(ctx, tripID)
		if err != nil {
			return err
		}

		current, err := s.repo.FindMember(ctx, tripID, targetID)
		if err != nil {
			return err
		}

		if current.Role == string(access.Owner) && role != access.Owner && len(owners) <= 1 {
			return apperror.Unprocessable("last_owner")
		}

		if err := s.repo.UpdateMemberRole(ctx, tripID, targetID, string(role)); err != nil {
			return err
		}

		member, err = s.repo.FindMember(ctx, tripID, targetID)
		return err
	})
	if err != nil {
		end(err)
		return nil, err
	}

	s.hub.UpdateRole(tripID, targetID, member.Role)
	s.hub.Publish(tripID, "member.changed", requestctx.ClientID(ctx), dto.MemberChange{
		Action: "role_changed", UserID: targetID, Member: member,
	})

	end(nil)
	return member, nil
}

func (s *TripServiceImpl) RemoveMember(ctx context.Context, actorID uuid.UUID, tripID uuid.UUID, targetID uuid.UUID) error {
	end := s.log.Start(ctx, "RemoveMember", "trip_id", tripID, "target", targetID)

	// Anyone may leave; removing somebody else takes an owner.
	required := access.Owner
	if actorID == targetID {
		required = access.Viewer
	}
	if _, err := s.access.Require(ctx, tripID, actorID, required); err != nil {
		end(err)
		return err
	}

	err := s.db.InTx(ctx, func(ctx context.Context) error {
		owners, err := s.repo.LockOwners(ctx, tripID)
		if err != nil {
			return err
		}

		current, err := s.repo.FindMember(ctx, tripID, targetID)
		if err != nil {
			return err
		}
		if current.Role == string(access.Owner) && len(owners) <= 1 {
			return apperror.Unprocessable("last_owner")
		}

		return s.repo.RemoveMember(ctx, tripID, targetID)
	})
	if err != nil {
		end(err)
		return err
	}

	s.hub.Publish(tripID, "member.changed", requestctx.ClientID(ctx), dto.MemberChange{
		Action: "removed", UserID: targetID,
	})
	s.hub.DisconnectUser(tripID, targetID)

	end(nil)
	return nil
}
