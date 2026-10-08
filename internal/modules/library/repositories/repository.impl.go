package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const (
	columns = "id, trip_id, kind, title, description_md, location_name, lat, lng, link_url, fixed_start_at, fixed_end_at, category, default_duration_min, banner_url, version, created_by, created_at, updated_at"
	// Lists skip the Markdown body, which can be 100 KB per item.
	listColumns = "id, trip_id, kind, title, ''::text AS description_md, location_name, lat, lng, link_url, fixed_start_at, fixed_end_at, category, default_duration_min, banner_url, version, created_by, created_at, updated_at"
)

type LibraryRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
	sb  sq.StatementBuilderType
}

func NewLibraryRepository(db *database.Database, appLogger *logger.Logger) LibraryRepository {
	return &LibraryRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.library"),
		sb:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// escapeLike makes user input match literally inside ILIKE.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func (r *LibraryRepositoryImpl) List(ctx context.Context, tripID uuid.UUID, filter ListFilter) ([]entities.LibraryItem, error) {
	builder := r.sb.Select(listColumns).From("library_items").Where(sq.Eq{"trip_id": tripID})

	if filter.Kind != "" {
		builder = builder.Where(sq.Eq{"kind": filter.Kind})
	}
	if filter.Query != "" {
		pattern := "%" + escapeLike(filter.Query) + "%"
		builder = builder.Where(sq.Or{
			sq.Expr("title ILIKE ?", pattern),
			sq.Expr("location_name ILIKE ?", pattern),
		})
	}
	if filter.CursorAt != nil && filter.CursorID != nil {
		builder = builder.Where(sq.Expr("(created_at, id) < (?, ?)", *filter.CursorAt, *filter.CursorID))
	}

	query, args, err := builder.OrderBy("created_at DESC", "id DESC").Limit(uint64(filter.Limit)).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.LibraryItem])
}

func (r *LibraryRepositoryImpl) Create(ctx context.Context, item entities.LibraryItem) (*entities.LibraryItem, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`INSERT INTO library_items (trip_id, kind, title, description_md, location_name, lat, lng, link_url, fixed_start_at, fixed_end_at,
		                            category, default_duration_min, banner_url, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING `+columns,
		item.TripID, item.Kind, item.Title, item.DescriptionMD, item.LocationName, item.Lat, item.Lng,
		item.LinkURL, item.FixedStartAt, item.FixedEndAt, item.Category, item.DefaultDurationMin, item.BannerURL, item.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.LibraryItem])
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *LibraryRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*entities.LibraryItem, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT `+columns+` FROM library_items WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.LibraryItem])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("library_item_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *LibraryRepositoryImpl) FindByIDs(ctx context.Context, tripID uuid.UUID, ids []uuid.UUID) ([]entities.LibraryItem, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT `+columns+` FROM library_items WHERE trip_id = $1 AND id = ANY($2)`, tripID, ids)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.LibraryItem])
}

// Update applies data only while the row still has the given version.
func (r *LibraryRepositoryImpl) Update(ctx context.Context, id uuid.UUID, version int, data map[string]any) (*entities.LibraryItem, error) {
	query, args, err := r.sb.Update("library_items").
		SetMap(data).
		Set("version", sq.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": id, "version": version}).
		Suffix("RETURNING " + columns).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	item, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.LibraryItem])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVersionConflict
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *LibraryRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM library_items WHERE id = $1`, id)
	return err
}

func (r *LibraryRepositoryImpl) BlocksOfItem(ctx context.Context, itemID uuid.UUID) ([]entities.ScheduleBlock, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT b.id, b.trip_id, b.library_item_id, b.title, b.note, b.hue, b.start_at, b.end_at, b.created_by, b.updated_by,
		        b.version, b.created_at, b.updated_at,
		        i.id AS item_id, i.kind AS item_kind, i.title AS item_title, i.location_name AS item_location_name, i.category AS item_category
		 FROM schedule_blocks b JOIN library_items i ON i.id = b.library_item_id
		 WHERE b.library_item_id = $1
		 ORDER BY b.start_at`,
		itemID,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.ScheduleBlock])
}

func (r *LibraryRepositoryImpl) DeleteBlocksOfItem(ctx context.Context, itemID uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM schedule_blocks WHERE library_item_id = $1`, itemID)
	return err
}
