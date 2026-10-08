package repositories

import (
	"context"
	"errors"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const (
	pgExclusionViolation = "23P01"

	blockSelect = `SELECT b.id, b.trip_id, b.library_item_id, b.title, b.note, b.hue, b.start_at, b.end_at,
		b.created_by, b.updated_by, b.version, b.created_at, b.updated_at,
		i.id AS item_id, i.kind AS item_kind, i.title AS item_title, i.location_name AS item_location_name, i.category AS item_category
		FROM schedule_blocks b LEFT JOIN library_items i ON i.id = b.library_item_id`
)

type ScheduleRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
	sb  sq.StatementBuilderType
}

func NewScheduleRepository(db *database.Database, appLogger *logger.Logger) ScheduleRepository {
	return &ScheduleRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.schedule"),
		sb:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
		return ErrOverlap
	}
	return err
}

func (r *ScheduleRepositoryImpl) List(ctx context.Context, tripID uuid.UUID, from *time.Time, to *time.Time) ([]entities.ScheduleBlock, error) {
	query := blockSelect + ` WHERE b.trip_id = $1`
	args := []any{tripID}

	// Blocks that overlap the window, not only those fully inside it.
	if to != nil {
		args = append(args, *to)
		query += ` AND b.start_at < $` + itoa(len(args))
	}
	if from != nil {
		args = append(args, *from)
		query += ` AND b.end_at > $` + itoa(len(args))
	}
	query += ` ORDER BY b.start_at`

	rows, err := r.db.Q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.ScheduleBlock])
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func (r *ScheduleRepositoryImpl) FindByID(ctx context.Context, id uuid.UUID) (*entities.ScheduleBlock, error) {
	rows, err := r.db.Q(ctx).Query(ctx, blockSelect+` WHERE b.id = $1`, id)
	if err != nil {
		return nil, err
	}

	block, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ScheduleBlock])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("block_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (r *ScheduleRepositoryImpl) Create(ctx context.Context, block entities.ScheduleBlock) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.Q(ctx).QueryRow(ctx,
		`INSERT INTO schedule_blocks (trip_id, library_item_id, title, note, hue, start_at, end_at, created_by, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		 RETURNING id`,
		block.TripID, block.LibraryItemID, block.Title, block.Note, block.Hue, block.StartAt, block.EndAt, block.CreatedBy,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, translate(err)
	}
	return id, nil
}

func (r *ScheduleRepositoryImpl) Update(ctx context.Context, id uuid.UUID, version int, data map[string]any) error {
	query, args, err := r.sb.Update("schedule_blocks").
		SetMap(data).
		Set("version", sq.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": id, "version": version}).
		ToSql()
	if err != nil {
		return err
	}

	tag, err := r.db.Q(ctx).Exec(ctx, query, args...)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *ScheduleRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM schedule_blocks WHERE id = $1`, id)
	return err
}

func (r *ScheduleRepositoryImpl) Conflicts(ctx context.Context, tripID uuid.UUID, start time.Time, end time.Time, excludeID *uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT id FROM schedule_blocks
		 WHERE trip_id = $1 AND start_at < $3 AND end_at > $2 AND ($4::uuid IS NULL OR id <> $4)
		 ORDER BY start_at`,
		tripID, start, end, excludeID,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *ScheduleRepositoryImpl) LibraryItemInTrip(ctx context.Context, itemID uuid.UUID, tripID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM library_items WHERE id = $1 AND trip_id = $2)`, itemID, tripID).Scan(&exists)
	return exists, err
}
