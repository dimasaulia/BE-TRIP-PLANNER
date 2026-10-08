package services

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/uploads/dto"
)

type UploadService interface {
	Upload(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, originalName string, src io.Reader) (*dto.UploadResponse, error)
	// Open returns the file of a trip image for a member; the caller closes it.
	Open(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, storedName string) (*os.File, *entities.Attachment, error)
	CleanupOrphans(ctx context.Context, minAge time.Duration) (int, error)
}
