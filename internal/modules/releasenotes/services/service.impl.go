package services

import (
	"context"
	"time"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/releasenotes/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

type ReleaseNoteServiceImpl struct {
	ReleaseNoteRepository repositories.ReleaseNoteRepository
	log                   *logger.LayerLogger
}

func NewReleaseNoteService(releaseNoteRepository repositories.ReleaseNoteRepository, appLogger *logger.Logger) ReleaseNoteService {
	return &ReleaseNoteServiceImpl{
		ReleaseNoteRepository: releaseNoteRepository,
		log:                   appLogger.Layer("service.release_notes"),
	}
}

func (s *ReleaseNoteServiceImpl) Find(ctx context.Context, limit uint64, offset uint64) ([]entities.ReleaseNote, error) {
	end := s.log.Start(ctx, "Find")
	releaseNotes, err := s.ReleaseNoteRepository.Find(ctx, limit, offset)
	end(err, "count", len(releaseNotes))
	return releaseNotes, err
}

func (s *ReleaseNoteServiceImpl) FindByID(ctx context.Context, id int64) (*entities.ReleaseNote, error) {
	end := s.log.Start(ctx, "FindByID", "id", id)
	releaseNote, err := s.ReleaseNoteRepository.FindByID(ctx, id)
	end(err)
	return releaseNote, err
}

func (s *ReleaseNoteServiceImpl) Create(ctx context.Context, request dto.CreateReleaseNoteRequest) (*entities.ReleaseNote, error) {
	end := s.log.Start(ctx, "Create")

	releaseDate, err := parseDate(request.ReleaseDate)
	if err != nil {
		end(err)
		return nil, err
	}

	isActive := request.IsActive
	if isActive == nil {
		defaultActive := int16(1)
		isActive = &defaultActive
	}

	releaseNote, err := s.ReleaseNoteRepository.Create(ctx, entities.ReleaseNote{
		Version:     request.Version,
		Category:    request.Category,
		Title:       request.Title,
		ParentID:    request.ParentID,
		Notes:       request.Notes,
		ReleaseDate: releaseDate,
		Visibility:  request.Visibility,
		IsActive:    isActive,
		CreatedBy:   request.CreatedBy,
	})
	end(err)
	return releaseNote, err
}

func (s *ReleaseNoteServiceImpl) Update(ctx context.Context, id int64, request dto.UpdateReleaseNoteRequest) (*entities.ReleaseNote, error) {
	end := s.log.Start(ctx, "Update", "id", id)

	data := map[string]any{}
	if request.Version != nil {
		data["version"] = *request.Version
	}
	if request.Category != nil {
		data["category"] = *request.Category
	}
	if request.Title != nil {
		data["title"] = *request.Title
	}
	if request.ParentID != nil {
		data["parent_id"] = *request.ParentID
	}
	if request.Notes != nil {
		data["notes"] = *request.Notes
	}
	if request.ReleaseDate != nil {
		releaseDate, err := parseDate(*request.ReleaseDate)
		if err != nil {
			end(err)
			return nil, err
		}
		data["release_date"] = releaseDate
	}
	if request.Visibility != nil {
		data["visibility"] = *request.Visibility
	}
	if request.IsActive != nil {
		data["is_active"] = *request.IsActive
	}
	if request.UpdatedBy != nil {
		data["updated_by"] = *request.UpdatedBy
	}

	releaseNote, err := s.ReleaseNoteRepository.Update(ctx, id, data)
	end(err)
	return releaseNote, err
}

func (s *ReleaseNoteServiceImpl) Delete(ctx context.Context, id int64, deletedBy *int64) error {
	end := s.log.Start(ctx, "Delete", "id", id)
	err := s.ReleaseNoteRepository.SoftDelete(ctx, id, deletedBy)
	end(err)
	return err
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}
