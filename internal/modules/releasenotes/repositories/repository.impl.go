package repositories

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

const tableName = "release_notes"

var ErrNotFound = errors.New("release note not found")

type ReleaseNoteRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
	sb  sq.StatementBuilderType
}

func NewReleaseNoteRepository(db *database.Database, appLogger *logger.Logger) ReleaseNoteRepository {
	return &ReleaseNoteRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.release_notes"),
		sb:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *ReleaseNoteRepositoryImpl) Find(ctx context.Context, limit uint64, offset uint64) ([]entities.ReleaseNote, error) {
	query, args, err := r.sb.Select(columns()...).
		From(tableName).
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("release_date DESC", "id DESC").
		Limit(limit).
		Offset(offset).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.ReleaseNote])
}

func (r *ReleaseNoteRepositoryImpl) FindByID(ctx context.Context, id int64) (*entities.ReleaseNote, error) {
	query, args, err := r.sb.Select(columns()...).
		From(tableName).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	releaseNote, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ReleaseNote])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &releaseNote, nil
}

func (r *ReleaseNoteRepositoryImpl) Create(ctx context.Context, releaseNote entities.ReleaseNote) (*entities.ReleaseNote, error) {
	values := map[string]any{
		"version":      releaseNote.Version,
		"category":     releaseNote.Category,
		"title":        releaseNote.Title,
		"parent_id":    releaseNote.ParentID,
		"notes":        releaseNote.Notes,
		"release_date": releaseNote.ReleaseDate,
		"visibility":   releaseNote.Visibility,
		"is_active":    releaseNote.IsActive,
		"created_by":   releaseNote.CreatedBy,
	}

	query, args, err := r.sb.Insert(tableName).
		SetMap(values).
		Suffix("RETURNING " + columnList()).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ReleaseNote])
	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *ReleaseNoteRepositoryImpl) Update(ctx context.Context, id int64, data map[string]any) (*entities.ReleaseNote, error) {
	data["updated_at"] = time.Now()

	query, args, err := r.sb.Update(tableName).
		SetMap(data).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + columnList()).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	updated, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ReleaseNote])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (r *ReleaseNoteRepositoryImpl) SoftDelete(ctx context.Context, id int64, deletedBy *int64) error {
	query, args, err := r.sb.Update(tableName).
		SetMap(map[string]any{
			"deleted_at": time.Now(),
			"deleted_by": deletedBy,
			"is_active":  int16(0),
		}).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return err
	}

	var deletedID int64
	if err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&deletedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

func columns() []string {
	return []string{
		"id",
		"version",
		"category",
		"title",
		"parent_id",
		"notes",
		"release_date",
		"visibility",
		"is_active",
		"created_at",
		"updated_at",
		"deleted_at",
		"created_by",
		"updated_by",
		"deleted_by",
	}
}

func columnList() string {
	return "id, version, category, title, parent_id, notes, release_date, visibility, is_active, created_at, updated_at, deleted_at, created_by, updated_by, deleted_by"
}
