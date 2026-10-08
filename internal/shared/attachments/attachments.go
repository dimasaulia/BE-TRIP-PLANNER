// Package attachments owns uploaded images: validation, disk + row bookkeeping,
// copying between trips (library export) and orphan cleanup.
package attachments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "golang.org/x/image/webp"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/config"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/storage"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const maxDimension = 4096

var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
}

const columns = "id, trip_id, uploaded_by, stored_name, original_name, mime, size_bytes, width, height, sha256, created_at"

type Service interface {
	// Upload validates and stores an image read from src.
	Upload(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, originalName string, src io.Reader) (*entities.Attachment, error)
	Find(ctx context.Context, tripID uuid.UUID, storedName string) (*entities.Attachment, error)
	// CopyToTrip duplicates the file and the row into another trip.
	CopyToTrip(ctx context.Context, source entities.Attachment, targetTrip uuid.UUID, userID uuid.UUID) (*entities.Attachment, error)
	// DeleteFiles removes copied files again when a surrounding transaction fails.
	DeleteFiles(attachments []entities.Attachment)
	// CleanupOrphans drops attachments no description references and older than minAge.
	CleanupOrphans(ctx context.Context, minAge time.Duration) (int, error)
}

type ServiceImpl struct {
	db       *database.Database
	store    *storage.Store
	maxBytes int64
	log      *logger.LayerLogger
}

func NewService(db *database.Database, store *storage.Store, cfg config.Config, appLogger *logger.Logger) Service {
	return &ServiceImpl{
		db:       db,
		store:    store,
		maxBytes: cfg.Upload.MaxBytes,
		log:      appLogger.Layer("shared.attachments"),
	}
}

func (s *ServiceImpl) Upload(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, originalName string, src io.Reader) (*entities.Attachment, error) {
	dir, err := s.store.EnsureTripDir(tripID)
	if err != nil {
		return nil, err
	}

	temp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	renamed := false
	defer func() {
		temp.Close()
		if !renamed {
			os.Remove(tempName)
		}
	}()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(src, s.maxBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, apperror.TooLarge("payload_too_large").With("max_bytes", s.maxBytes)
		}
		return nil, apperror.BadRequest("invalid_upload").Wrap(err)
	}
	if size > s.maxBytes {
		return nil, apperror.TooLarge("payload_too_large").With("max_bytes", s.maxBytes)
	}
	if size == 0 {
		return nil, apperror.BadRequest("invalid_image")
	}

	// Trust the bytes, never the extension or the client's Content-Type.
	head := make([]byte, 512)
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	n, _ := io.ReadFull(temp, head)
	mime := http.DetectContentType(head[:n])
	extension, ok := extensions[mime]
	if !ok {
		return nil, apperror.UnsupportedMedia("unsupported_image_type")
	}

	// DecodeConfig reads only the header, so a decompression bomb is rejected
	// by its declared dimensions without allocating the pixels.
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(temp)
	if err != nil {
		return nil, apperror.BadRequest("invalid_image").Wrap(err)
	}
	if config.Width > maxDimension || config.Height > maxDimension {
		return nil, apperror.Unprocessable("image_dimensions_exceeded").
			With("max_width", maxDimension).With("max_height", maxDimension)
	}

	if err := temp.Close(); err != nil {
		return nil, err
	}

	storedName := uuid.NewString() + "." + extension
	if err := os.Rename(tempName, filepath.Join(dir, storedName)); err != nil {
		return nil, err
	}
	renamed = true

	attachment, err := s.insert(ctx, entities.Attachment{
		TripID:       tripID,
		UploadedBy:   &userID,
		StoredName:   storedName,
		OriginalName: originalName,
		Mime:         mime,
		SizeBytes:    size,
		Width:        config.Width,
		Height:       config.Height,
		SHA256:       hex.EncodeToString(hasher.Sum(nil)),
	})
	if err != nil {
		_ = s.store.Remove(tripID, storedName)
		return nil, err
	}

	return attachment, nil
}

func (s *ServiceImpl) insert(ctx context.Context, attachment entities.Attachment) (*entities.Attachment, error) {
	rows, err := s.db.Q(ctx).Query(ctx,
		`INSERT INTO attachments (trip_id, uploaded_by, stored_name, original_name, mime, size_bytes, width, height, sha256)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+columns,
		attachment.TripID, attachment.UploadedBy, attachment.StoredName, attachment.OriginalName,
		attachment.Mime, attachment.SizeBytes, attachment.Width, attachment.Height, attachment.SHA256,
	)
	if err != nil {
		return nil, err
	}

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Attachment])
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *ServiceImpl) Find(ctx context.Context, tripID uuid.UUID, storedName string) (*entities.Attachment, error) {
	rows, err := s.db.Q(ctx).Query(ctx,
		`SELECT `+columns+` FROM attachments WHERE trip_id = $1 AND stored_name = $2`,
		tripID, storedName,
	)
	if err != nil {
		return nil, err
	}

	attachment, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Attachment])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("file_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (s *ServiceImpl) CopyToTrip(ctx context.Context, source entities.Attachment, targetTrip uuid.UUID, userID uuid.UUID) (*entities.Attachment, error) {
	extension := filepath.Ext(source.StoredName)
	storedName := uuid.NewString() + extension

	if err := s.store.Copy(source.TripID, source.StoredName, targetTrip, storedName); err != nil {
		return nil, err
	}

	copied, err := s.insert(ctx, entities.Attachment{
		TripID:       targetTrip,
		UploadedBy:   &userID,
		StoredName:   storedName,
		OriginalName: source.OriginalName,
		Mime:         source.Mime,
		SizeBytes:    source.SizeBytes,
		Width:        source.Width,
		Height:       source.Height,
		SHA256:       source.SHA256,
	})
	if err != nil {
		_ = s.store.Remove(targetTrip, storedName)
		return nil, err
	}
	return copied, nil
}

func (s *ServiceImpl) DeleteFiles(attachments []entities.Attachment) {
	for _, attachment := range attachments {
		_ = s.store.Remove(attachment.TripID, attachment.StoredName)
	}
}

func (s *ServiceImpl) CleanupOrphans(ctx context.Context, minAge time.Duration) (int, error) {
	rows, err := s.db.Pool.Query(ctx,
		`DELETE FROM attachments a
		 WHERE a.created_at < $1
		   AND NOT EXISTS (
		       SELECT 1 FROM library_items li
		       WHERE li.trip_id = a.trip_id
		         AND (position(a.stored_name IN li.description_md) > 0
		              OR position(a.stored_name IN COALESCE(li.banner_url, '')) > 0)
		   )
		   AND NOT EXISTS (
		       SELECT 1 FROM trips t
		       WHERE t.id = a.trip_id AND position(a.stored_name IN COALESCE(t.banner_url, '')) > 0
		   )
		 RETURNING `+columns,
		time.Now().Add(-minAge),
	)
	if err != nil {
		return 0, err
	}

	removed, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Attachment])
	if err != nil {
		return 0, err
	}

	s.DeleteFiles(removed)
	return len(removed), nil
}

// FileURL is the public path of a stored file; it is what Markdown embeds.
func FileURL(tripID uuid.UUID, storedName string) string {
	return "/api/v1/files/" + tripID.String() + "/" + storedName
}

// FilePattern matches FileURL occurrences of one trip inside Markdown and
// captures the stored name.
func FilePattern(tripID uuid.UUID) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta("/api/v1/files/"+tripID.String()+"/") + `([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(?:jpg|png|webp|gif))`)
}
