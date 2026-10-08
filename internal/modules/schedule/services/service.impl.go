package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/schedule/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/syncqueue"
	"github.com/open-suite/boilerplate-golang/internal/shared/timeutil"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

func itoa(value int) string {
	return strconv.Itoa(value)
}

type ScheduleServiceImpl struct {
	repo        repositories.ScheduleRepository
	db          *database.Database
	access      access.Service
	hub         realtime.Publisher
	queue       syncqueue.Queue
	granularity int
	log         *logger.LayerLogger
}

func NewScheduleService(
	repo repositories.ScheduleRepository,
	db *database.Database,
	accessService access.Service,
	hub realtime.Publisher,
	queue syncqueue.Queue,
	cfg config.Config,
	appLogger *logger.Logger,
) ScheduleService {
	return &ScheduleServiceImpl{
		repo:        repo,
		db:          db,
		access:      accessService,
		hub:         hub,
		queue:       queue,
		granularity: cfg.Schedule.GranularityMin,
		log:         appLogger.Layer("service.schedule"),
	}
}

func (s *ScheduleServiceImpl) List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, from string, to string) (*dto.ListResponse, error) {
	end := s.log.Start(ctx, "List", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return nil, err
	}

	var fromTime, toTime *time.Time
	if from != "" {
		parsed, err := timeutil.ParseUTC(from)
		if err != nil {
			end(err)
			return nil, err
		}
		fromTime = &parsed
	}
	if to != "" {
		parsed, err := timeutil.ParseUTC(to)
		if err != nil {
			end(err)
			return nil, err
		}
		toTime = &parsed
	}

	blocks, err := s.repo.List(ctx, tripID, fromTime, toTime)
	if err != nil {
		end(err)
		return nil, err
	}

	response := &dto.ListResponse{Items: make([]entities.BlockView, 0, len(blocks))}
	for _, block := range blocks {
		response.Items = append(response.Items, block.View())
	}

	end(nil, "count", len(response.Items))
	return response, nil
}

// checkContent enforces "an item or a title" and that the item belongs to the trip.
func (s *ScheduleServiceImpl) checkContent(ctx context.Context, tripID uuid.UUID, itemID *uuid.UUID, title *string, note string, hue *int) error {
	validator := validation.New()
	validator.Check(itemID != nil || title != nil, "title", "library_item_id or title is required")
	if title != nil {
		validator.Check(utf8.RuneCountInString(*title) <= 200, "title", "must be at most 200 characters")
	}
	validator.Check(utf8.RuneCountInString(note) <= 2000, "note", "must be at most 2000 characters")
	validator.Check(hue == nil || (*hue >= 0 && *hue <= 359), "hue", "must be between 0 and 359")
	if err := validator.Err(); err != nil {
		return err
	}

	if itemID != nil {
		inTrip, err := s.repo.LibraryItemInTrip(ctx, *itemID, tripID)
		if err != nil {
			return err
		}
		if !inTrip {
			return apperror.NotFound("library_item_not_found")
		}
	}
	return nil
}

// overlapError builds the 422 block_overlap response naming the colliding blocks.
func (s *ScheduleServiceImpl) overlapError(ctx context.Context, tripID uuid.UUID, start time.Time, end time.Time, exclude *uuid.UUID) error {
	conflicts, err := s.repo.Conflicts(ctx, tripID, start, end, exclude)
	if err != nil {
		return err
	}
	return apperror.Unprocessable("block_overlap").With("conflicts", conflicts)
}

func (s *ScheduleServiceImpl) Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateBlockRequest) (*entities.BlockView, error) {
	end := s.log.Start(ctx, "Create", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	validator := validation.New()
	validator.Check(request.StartAt != nil, "start_at", "is required")
	validator.Check(request.EndAt != nil, "end_at", "is required")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	title := normalizeTitle(request.Title)
	if err := s.checkContent(ctx, tripID, request.LibraryItemID, title, request.Note, request.Hue); err != nil {
		end(err)
		return nil, err
	}

	trip, err := s.access.Trip(ctx, tripID)
	if err != nil {
		end(err)
		return nil, err
	}
	start, finish := request.StartAt.Time, request.EndAt.Time
	if err := ValidateBlockTimes(start, finish, *trip, s.granularity); err != nil {
		end(err)
		return nil, err
	}

	conflicts, err := s.repo.Conflicts(ctx, tripID, start, finish, nil)
	if err != nil {
		end(err)
		return nil, err
	}
	if len(conflicts) > 0 {
		err := apperror.Unprocessable("block_overlap").With("conflicts", conflicts)
		end(err)
		return nil, err
	}

	var view *entities.BlockView
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		id, err := s.repo.Create(ctx, entities.ScheduleBlock{
			TripID:        tripID,
			LibraryItemID: request.LibraryItemID,
			Title:         title,
			Note:          request.Note,
			Hue:           request.Hue,
			StartAt:       start,
			EndAt:         finish,
			CreatedBy:     &userID,
		})
		if err != nil {
			return err
		}

		if err := s.queue.EnqueueBlock(ctx, tripID, id, entities.SyncOpUpsert); err != nil {
			return err
		}

		block, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		created := block.View()
		view = &created
		return nil
	})
	if errors.Is(err, repositories.ErrOverlap) {
		err = s.overlapError(ctx, tripID, start, finish, nil)
	}
	if err != nil {
		end(err)
		return nil, err
	}

	s.hub.Publish(tripID, "block.created", requestctx.ClientID(ctx), view)

	end(nil, "block_id", view.ID)
	return view, nil
}

func (s *ScheduleServiceImpl) Update(ctx context.Context, userID uuid.UUID, blockID uuid.UUID, request dto.UpdateBlockRequest) (*entities.BlockView, error) {
	end := s.log.Start(ctx, "Update", "block_id", blockID)

	block, err := s.repo.FindByID(ctx, blockID)
	if err != nil {
		end(err)
		return nil, err
	}
	if _, err := access.RequireFor(ctx, s.access, block.TripID, userID, access.Editor, "block_not_found"); err != nil {
		end(err)
		return nil, err
	}

	validator := validation.New()
	validator.Check(request.Version != nil, "version", "is required")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	itemID := block.LibraryItemID
	title := block.Title
	note := block.Note
	hue := block.Hue
	start, finish := block.StartAt, block.EndAt
	data := map[string]any{"updated_by": userID}

	if request.LibraryItemID.Set {
		itemID = request.LibraryItemID.Ptr()
		data["library_item_id"] = itemID
	}
	if request.Title.Set {
		title = nil
		if !request.Title.Null {
			title = normalizeTitle(&request.Title.Value)
		}
		data["title"] = title
	}
	if request.Note.Set {
		validator := validation.New()
		validator.Check(!request.Note.Null, "note", "cannot be null")
		if err := validator.Err(); err != nil {
			end(err)
			return nil, err
		}
		note = request.Note.Value
		data["note"] = note
	}
	if request.Hue.Set {
		hue = request.Hue.Ptr()
		data["hue"] = hue
	}
	timesChanged := false
	if request.StartAt != nil {
		start, timesChanged = request.StartAt.Time, true
		data["start_at"] = start
	}
	if request.EndAt != nil {
		finish, timesChanged = request.EndAt.Time, true
		data["end_at"] = finish
	}
	if len(data) == 1 {
		err := validation.New().ErrWith("body", "at least one field to change is required")
		end(err)
		return nil, err
	}

	if err := s.checkContent(ctx, block.TripID, itemID, title, note, hue); err != nil {
		end(err)
		return nil, err
	}

	if timesChanged {
		trip, err := s.access.Trip(ctx, block.TripID)
		if err != nil {
			end(err)
			return nil, err
		}
		if err := ValidateBlockTimes(start, finish, *trip, s.granularity); err != nil {
			end(err)
			return nil, err
		}

		conflicts, err := s.repo.Conflicts(ctx, block.TripID, start, finish, &blockID)
		if err != nil {
			end(err)
			return nil, err
		}
		if len(conflicts) > 0 {
			err := apperror.Unprocessable("block_overlap").With("conflicts", conflicts)
			end(err)
			return nil, err
		}
	}

	var view *entities.BlockView
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, blockID, *request.Version, data); err != nil {
			return err
		}
		if err := s.queue.EnqueueBlock(ctx, block.TripID, blockID, entities.SyncOpUpsert); err != nil {
			return err
		}

		updated, err := s.repo.FindByID(ctx, blockID)
		if err != nil {
			return err
		}
		result := updated.View()
		view = &result
		return nil
	})
	switch {
	case errors.Is(err, repositories.ErrVersionConflict):
		// The caller lost the race; hand back the winning state to merge or refetch.
		current, findErr := s.repo.FindByID(ctx, blockID)
		if findErr != nil {
			err = findErr
		} else {
			err = apperror.Conflict("version_conflict").With("current", current.View())
		}
	case errors.Is(err, repositories.ErrOverlap):
		err = s.overlapError(ctx, block.TripID, start, finish, &blockID)
	}
	if err != nil {
		end(err)
		return nil, err
	}

	s.hub.Publish(block.TripID, "block.updated", requestctx.ClientID(ctx), view)

	end(nil, "version", view.Version)
	return view, nil
}

func (s *ScheduleServiceImpl) Delete(ctx context.Context, userID uuid.UUID, blockID uuid.UUID) error {
	end := s.log.Start(ctx, "Delete", "block_id", blockID)

	block, err := s.repo.FindByID(ctx, blockID)
	if err != nil {
		end(err)
		return err
	}
	if _, err := access.RequireFor(ctx, s.access, block.TripID, userID, access.Editor, "block_not_found"); err != nil {
		end(err)
		return err
	}

	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Delete(ctx, blockID); err != nil {
			return err
		}
		return s.queue.EnqueueBlock(ctx, block.TripID, blockID, entities.SyncOpDelete)
	})
	if err != nil {
		end(err)
		return err
	}

	s.hub.Publish(block.TripID, "block.deleted", requestctx.ClientID(ctx), block.View())

	end(nil)
	return nil
}

// normalizeTitle turns a blank title into nil so the block follows its library item.
func normalizeTitle(title *string) *string {
	if title == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*title)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
