package services

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/uploads/dto"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/storage"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/attachments"
)

type UploadServiceImpl struct {
	access      access.Service
	attachments attachments.Service
	store       *storage.Store
	log         *logger.LayerLogger
}

func NewUploadService(accessService access.Service, attachmentService attachments.Service, store *storage.Store, appLogger *logger.Logger) UploadService {
	return &UploadServiceImpl{
		access:      accessService,
		attachments: attachmentService,
		store:       store,
		log:         appLogger.Layer("service.uploads"),
	}
}

func (s *UploadServiceImpl) Upload(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, originalName string, src io.Reader) (*dto.UploadResponse, error) {
	end := s.log.Start(ctx, "Upload", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	attachment, err := s.attachments.Upload(ctx, tripID, userID, originalName, src)
	if err != nil {
		end(err)
		return nil, err
	}

	end(nil, "attachment_id", attachment.ID, "size", attachment.SizeBytes)
	return &dto.UploadResponse{
		ID:     attachment.ID,
		URL:    attachments.FileURL(tripID, attachment.StoredName),
		Width:  attachment.Width,
		Height: attachment.Height,
	}, nil
}

func (s *UploadServiceImpl) Open(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, storedName string) (*os.File, *entities.Attachment, error) {
	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		return nil, nil, err
	}
	if !storage.ValidStoredName(storedName) {
		return nil, nil, apperror.NotFound("file_not_found")
	}

	attachment, err := s.attachments.Find(ctx, tripID, storedName)
	if err != nil {
		return nil, nil, err
	}

	file, err := s.store.Open(tripID, storedName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, apperror.NotFound("file_not_found")
		}
		return nil, nil, err
	}

	return file, attachment, nil
}

func (s *UploadServiceImpl) CleanupOrphans(ctx context.Context, minAge time.Duration) (int, error) {
	end := s.log.Start(ctx, "CleanupOrphans")
	removed, err := s.attachments.CleanupOrphans(ctx, minAge)
	end(err, "removed", removed)
	return removed, err
}
