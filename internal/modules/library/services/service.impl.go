package services

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/library/dto"
	"github.com/open-suite/boilerplate-golang/internal/modules/library/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/platform/realtime"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
	"github.com/open-suite/boilerplate-golang/internal/shared/attachments"
	"github.com/open-suite/boilerplate-golang/internal/shared/requestctx"
	"github.com/open-suite/boilerplate-golang/internal/shared/syncqueue"
	"github.com/open-suite/boilerplate-golang/internal/shared/validation"
)

const (
	defaultPageSize    = 50
	maxPageSize        = 100
	maxExportItems     = 50
	defaultDurationMin = 60
)

type LibraryServiceImpl struct {
	repo        repositories.LibraryRepository
	db          *database.Database
	access      access.Service
	hub         realtime.Publisher
	attachments attachments.Service
	queue       syncqueue.Queue
	log         *logger.LayerLogger
}

func NewLibraryService(
	repo repositories.LibraryRepository,
	db *database.Database,
	accessService access.Service,
	hub realtime.Publisher,
	attachmentService attachments.Service,
	queue syncqueue.Queue,
	appLogger *logger.Logger,
) LibraryService {
	return &LibraryServiceImpl{
		repo:        repo,
		db:          db,
		access:      accessService,
		hub:         hub,
		attachments: attachmentService,
		queue:       queue,
		log:         appLogger.Layer("service.library"),
	}
}

func encodeCursor(item entities.LibraryItem) string {
	raw := item.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + item.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (*time.Time, *uuid.UUID, error) {
	invalid := apperror.BadRequest("invalid_cursor")

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, nil, invalid
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, nil, invalid
	}

	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, nil, invalid
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, nil, invalid
	}
	return &at, &id, nil
}

func (s *LibraryServiceImpl) List(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, kind string, query string, cursor string, limit int) (*dto.ListResponse, error) {
	end := s.log.Start(ctx, "List", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Viewer); err != nil {
		end(err)
		return nil, err
	}

	if kind != "" && kind != "destination" && kind != "event" {
		err := validation.New().ErrWith("kind", "must be destination or event")
		end(err)
		return nil, err
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	filter := repositories.ListFilter{Kind: kind, Query: strings.TrimSpace(query), Limit: limit + 1}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			end(err)
			return nil, err
		}
		filter.CursorAt, filter.CursorID = at, id
	}

	items, err := s.repo.List(ctx, tripID, filter)
	if err != nil {
		end(err)
		return nil, err
	}

	response := &dto.ListResponse{Items: make([]entities.LibrarySummary, 0, len(items))}
	if len(items) > limit {
		items = items[:limit]
		next := encodeCursor(items[len(items)-1])
		response.NextCursor = &next
	}
	for _, item := range items {
		response.Items = append(response.Items, item.Summary())
	}

	end(nil, "count", len(response.Items))
	return response, nil
}

func (s *LibraryServiceImpl) Create(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.CreateItemRequest) (*entities.LibraryItem, error) {
	end := s.log.Start(ctx, "Create", "trip_id", tripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	candidate := entities.LibraryItem{
		TripID:        tripID,
		Kind:          request.Kind,
		Title:         strings.TrimSpace(request.Title),
		DescriptionMD: request.DescriptionMD,
		LocationName:  request.LocationName,
		Lat:           request.Lat,
		Lng:           request.Lng,
		LinkURL:       request.LinkURL,
		Category:      normalizeCategory(request.Category),
		BannerURL:     request.BannerURL,
		CreatedBy:     &userID,
	}
	candidate.DefaultDurationMin = defaultDurationMin
	if request.DefaultDurationMin != nil {
		candidate.DefaultDurationMin = *request.DefaultDurationMin
	}
	if request.FixedStartAt != nil {
		candidate.FixedStartAt = &request.FixedStartAt.Time
	}
	if request.FixedEndAt != nil {
		candidate.FixedEndAt = &request.FixedEndAt.Time
	}
	if err := validateItem(candidate); err != nil {
		end(err)
		return nil, err
	}
	if candidate.BannerURL != nil {
		if err := s.validateBanner(ctx, tripID, *candidate.BannerURL); err != nil {
			end(err)
			return nil, err
		}
	}

	created, err := s.repo.Create(ctx, candidate)
	if err != nil {
		end(err)
		return nil, err
	}

	s.hub.Publish(tripID, "library.created", requestctx.ClientID(ctx), created.Summary())

	end(nil, "item_id", created.ID)
	return created, nil
}

func (s *LibraryServiceImpl) Get(ctx context.Context, userID uuid.UUID, itemID uuid.UUID) (*entities.LibraryItem, error) {
	end := s.log.Start(ctx, "Get", "item_id", itemID)

	item, err := s.repo.FindByID(ctx, itemID)
	if err != nil {
		end(err)
		return nil, err
	}
	if _, err := access.RequireFor(ctx, s.access, item.TripID, userID, access.Viewer, "library_item_not_found"); err != nil {
		end(err)
		return nil, err
	}

	end(nil)
	return item, nil
}

func (s *LibraryServiceImpl) Update(ctx context.Context, userID uuid.UUID, itemID uuid.UUID, request dto.UpdateItemRequest) (*entities.LibraryItem, error) {
	end := s.log.Start(ctx, "Update", "item_id", itemID)

	item, err := s.repo.FindByID(ctx, itemID)
	if err != nil {
		end(err)
		return nil, err
	}
	if _, err := access.RequireFor(ctx, s.access, item.TripID, userID, access.Editor, "library_item_not_found"); err != nil {
		end(err)
		return nil, err
	}

	validator := validation.New()
	validator.Check(request.Version != nil, "version", "is required")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	merged, data, err := mergeUpdate(*item, request)
	if err != nil {
		end(err)
		return nil, err
	}
	if len(data) == 0 {
		err := validation.New().ErrWith("body", "at least one field to change is required")
		end(err)
		return nil, err
	}
	if request.BannerURL.Set && merged.BannerURL != nil {
		if err := s.validateBanner(ctx, item.TripID, *merged.BannerURL); err != nil {
			end(err)
			return nil, err
		}
	}

	var updated *entities.LibraryItem
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		result, err := s.repo.Update(ctx, itemID, *request.Version, data)
		if errors.Is(err, repositories.ErrVersionConflict) {
			current, findErr := s.repo.FindByID(ctx, itemID)
			if findErr != nil {
				return findErr
			}
			return apperror.Conflict("version_conflict").With("current", current)
		}
		if err != nil {
			return err
		}
		updated = result

		if calendarChanged(*item, merged) {
			return s.queue.EnqueueItemBlocks(ctx, itemID)
		}
		return nil
	})
	if err != nil {
		end(err)
		return nil, err
	}

	if summaryChanged(*item, *updated) {
		s.hub.Publish(updated.TripID, "library.updated", requestctx.ClientID(ctx), updated.Summary())
	}

	end(nil, "version", updated.Version)
	return updated, nil
}

// mergeUpdate overlays the request on the stored item, validates the result and
// returns the column map to write.
func mergeUpdate(item entities.LibraryItem, request dto.UpdateItemRequest) (entities.LibraryItem, map[string]any, error) {
	merged := item
	data := map[string]any{}
	validator := validation.New()

	if request.Kind.Set {
		validator.Check(!request.Kind.Null, "kind", "cannot be null")
		merged.Kind = request.Kind.Value
		data["kind"] = merged.Kind
	}
	if request.Title.Set {
		validator.Check(!request.Title.Null, "title", "cannot be null")
		merged.Title = strings.TrimSpace(request.Title.Value)
		data["title"] = merged.Title
	}
	if request.DescriptionMD.Set {
		validator.Check(!request.DescriptionMD.Null, "description_md", "cannot be null")
		merged.DescriptionMD = request.DescriptionMD.Value
		data["description_md"] = merged.DescriptionMD
	}
	if request.LocationName.Set {
		merged.LocationName = request.LocationName.Ptr()
		data["location_name"] = merged.LocationName
	}
	if request.Lat.Set {
		merged.Lat = request.Lat.Ptr()
		data["lat"] = merged.Lat
	}
	if request.Lng.Set {
		merged.Lng = request.Lng.Ptr()
		data["lng"] = merged.Lng
	}
	if request.LinkURL.Set {
		merged.LinkURL = request.LinkURL.Ptr()
		data["link_url"] = merged.LinkURL
	}
	if request.FixedStartAt.Set {
		merged.FixedStartAt = nil
		if !request.FixedStartAt.Null {
			value := request.FixedStartAt.Value.Time
			merged.FixedStartAt = &value
		}
		data["fixed_start_at"] = merged.FixedStartAt
	}
	if request.FixedEndAt.Set {
		merged.FixedEndAt = nil
		if !request.FixedEndAt.Null {
			value := request.FixedEndAt.Value.Time
			merged.FixedEndAt = &value
		}
		data["fixed_end_at"] = merged.FixedEndAt
	}

	if request.Category.Set {
		merged.Category = normalizeCategory(request.Category.Ptr())
		data["category"] = merged.Category
	}
	if request.DefaultDurationMin.Set {
		validator.Check(!request.DefaultDurationMin.Null, "default_duration_min", "cannot be null")
		merged.DefaultDurationMin = request.DefaultDurationMin.Value
		data["default_duration_min"] = merged.DefaultDurationMin
	}
	if request.BannerURL.Set {
		merged.BannerURL = request.BannerURL.Ptr()
		data["banner_url"] = merged.BannerURL
	}

	if err := validator.Err(); err != nil {
		return merged, nil, err
	}
	if err := validateItem(merged); err != nil {
		return merged, nil, err
	}
	return merged, data, nil
}

func (s *LibraryServiceImpl) Delete(ctx context.Context, userID uuid.UUID, itemID uuid.UUID, force bool) error {
	end := s.log.Start(ctx, "Delete", "item_id", itemID, "force", force)

	item, err := s.repo.FindByID(ctx, itemID)
	if err != nil {
		end(err)
		return err
	}
	if _, err := access.RequireFor(ctx, s.access, item.TripID, userID, access.Editor, "library_item_not_found"); err != nil {
		end(err)
		return err
	}

	var removedBlocks []entities.ScheduleBlock
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		blocks, err := s.repo.BlocksOfItem(ctx, itemID)
		if err != nil {
			return err
		}

		if len(blocks) > 0 {
			if !force {
				return apperror.Conflict("library_item_in_use").With("block_count", len(blocks))
			}

			for _, block := range blocks {
				if err := s.queue.EnqueueBlock(ctx, block.TripID, block.ID, entities.SyncOpDelete); err != nil {
					return err
				}
			}
			if err := s.repo.DeleteBlocksOfItem(ctx, itemID); err != nil {
				return err
			}
			removedBlocks = blocks
		}

		return s.repo.Delete(ctx, itemID)
	})
	if err != nil {
		end(err)
		return err
	}

	origin := requestctx.ClientID(ctx)
	for _, block := range removedBlocks {
		s.hub.Publish(item.TripID, "block.deleted", origin, block.View())
	}
	s.hub.Publish(item.TripID, "library.deleted", origin, item.Summary())

	end(nil, "blocks_removed", len(removedBlocks))
	return nil
}

func (s *LibraryServiceImpl) Export(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, request dto.ExportRequest) (*dto.ExportResponse, error) {
	end := s.log.Start(ctx, "Export", "trip_id", tripID, "target", request.TargetTripID)

	if _, err := s.access.Require(ctx, tripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	validator := validation.New()
	validator.Check(request.TargetTripID != uuid.Nil, "target_trip_id", "is required")
	validator.Check(request.TargetTripID != tripID, "target_trip_id", "must be a different trip")
	validator.Check(len(request.ItemIDs) >= 1 && len(request.ItemIDs) <= maxExportItems, "item_ids", "must contain between 1 and 50 items")
	if err := validator.Err(); err != nil {
		end(err)
		return nil, err
	}

	if _, err := s.access.Require(ctx, request.TargetTripID, userID, access.Editor); err != nil {
		end(err)
		return nil, err
	}

	ids := uniqueIDs(request.ItemIDs)

	var (
		created []entities.LibraryItem
		copied  []entities.Attachment
	)

	// One transaction: if a single item fails, none of them are exported and
	// the files copied so far are removed again.
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		sources, err := s.repo.FindByIDs(ctx, tripID, ids)
		if err != nil {
			return err
		}
		if len(sources) != len(ids) {
			return apperror.NotFound("library_item_not_found").With("missing", missingIDs(ids, sources))
		}

		byID := make(map[uuid.UUID]entities.LibraryItem, len(sources))
		for _, source := range sources {
			byID[source.ID] = source
		}

		pattern := attachments.FilePattern(tripID)
		for _, id := range ids {
			source := byID[id]

			description, newFiles, err := s.rewriteImages(ctx, pattern, source.DescriptionMD, tripID, request.TargetTripID, userID)
			copied = append(copied, newFiles...)
			if err != nil {
				return err
			}

			duplicate := source
			duplicate.TripID = request.TargetTripID
			duplicate.DescriptionMD = description
			duplicate.CreatedBy = &userID

			if source.BannerURL != nil {
				banner, bannerFiles, err := s.rewriteImages(ctx, pattern, *source.BannerURL, tripID, request.TargetTripID, userID)
				copied = append(copied, bannerFiles...)
				if err != nil {
					return err
				}
				duplicate.BannerURL = &banner
			}

			item, err := s.repo.Create(ctx, duplicate)
			if err != nil {
				return err
			}
			created = append(created, *item)
		}
		return nil
	})
	if err != nil {
		s.attachments.DeleteFiles(copied)
		end(err)
		return nil, err
	}

	origin := requestctx.ClientID(ctx)
	for _, item := range created {
		s.hub.Publish(request.TargetTripID, "library.created", origin, item.Summary())
	}

	end(nil, "count", len(created))
	return &dto.ExportResponse{Items: created}, nil
}

// rewriteImages copies every image the description embeds into the target trip
// and points the Markdown at the copies.
func (s *LibraryServiceImpl) rewriteImages(ctx context.Context, pattern *regexp.Regexp, description string, sourceTrip uuid.UUID, targetTrip uuid.UUID, userID uuid.UUID) (string, []entities.Attachment, error) {
	mapping := map[string]string{}
	var copied []entities.Attachment

	for _, match := range pattern.FindAllStringSubmatch(description, -1) {
		storedName := match[1]
		if _, done := mapping[storedName]; done {
			continue
		}

		source, err := s.attachments.Find(ctx, sourceTrip, storedName)
		if err != nil {
			if appErr, ok := apperror.As(err); ok && appErr.Status == 404 {
				continue // a dangling link is copied as is
			}
			return "", copied, err
		}

		duplicate, err := s.attachments.CopyToTrip(ctx, *source, targetTrip, userID)
		if err != nil {
			return "", copied, err
		}
		copied = append(copied, *duplicate)
		mapping[storedName] = duplicate.StoredName
	}

	for oldName, newName := range mapping {
		description = strings.ReplaceAll(description,
			attachments.FileURL(sourceTrip, oldName),
			attachments.FileURL(targetTrip, newName))
	}
	return description, copied, nil
}

// normalizeCategory trims the label and maps an empty one to "no category".
func normalizeCategory(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.Join(strings.Fields(*value), " ")
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validateBanner accepts an external http(s) image URL or an image uploaded to
// the item's own trip, the same rule as the trip banner.
func (s *LibraryServiceImpl) validateBanner(ctx context.Context, tripID uuid.UUID, value string) error {
	invalid := validation.New().ErrWith("banner_url", "must be an http(s) URL or an image uploaded to this trip")

	if match := attachments.FilePattern(tripID).FindStringSubmatch(value); match != nil && value == attachments.FileURL(tripID, match[1]) {
		if _, err := s.attachments.Find(ctx, tripID, match[1]); err != nil {
			return invalid
		}
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(value) > 2048 {
		return invalid
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func missingIDs(wanted []uuid.UUID, found []entities.LibraryItem) []uuid.UUID {
	present := make(map[uuid.UUID]struct{}, len(found))
	for _, item := range found {
		present[item.ID] = struct{}{}
	}

	var missing []uuid.UUID
	for _, id := range wanted {
		if _, ok := present[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}
