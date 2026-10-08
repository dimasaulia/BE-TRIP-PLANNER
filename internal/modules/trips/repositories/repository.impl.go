package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/modules/trips/dto"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/access"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const memberSelect = `SELECT m.trip_id, m.user_id, m.role, m.joined_at, u.name, u.email, u.avatar_url
	FROM trip_members m JOIN users u ON u.id = m.user_id`

type TripRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
}

func NewTripRepository(db *database.Database, appLogger *logger.Logger) TripRepository {
	return &TripRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.trips"),
	}
}

func (r *TripRepositoryImpl) Create(ctx context.Context, trip entities.Trip) (*entities.Trip, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`INSERT INTO trips (name, description, start_date, end_date, timezone, banner_url, created_by)
		 VALUES ($1, $2, $3::date, $4::date, $5, $6, $7)
		 RETURNING `+access.TripColumns,
		trip.Name, trip.Description, trip.StartDate, trip.EndDate, trip.Timezone, trip.BannerURL, trip.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Trip])
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *TripRepositoryImpl) AddMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO trip_members (trip_id, user_id, role) VALUES ($1, $2, $3)`,
		tripID, userID, role,
	)
	return err
}

func (r *TripRepositoryImpl) ListForUser(ctx context.Context, userID uuid.UUID) ([]TripListRow, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT `+access.TripColumns+`, m.role,
		        (SELECT COUNT(*) FROM trip_members c WHERE c.trip_id = trips.id)::int AS member_count
		 FROM trips JOIN trip_members m ON m.trip_id = trips.id
		 WHERE m.user_id = $1
		 ORDER BY trips.start_date DESC, trips.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[TripListRow])
}

func (r *TripRepositoryImpl) Find(ctx context.Context, id uuid.UUID) (*entities.Trip, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT `+access.TripColumns+` FROM trips WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	trip, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Trip])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("trip_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepositoryImpl) Update(ctx context.Context, id uuid.UUID, data map[string]any) (*entities.Trip, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`UPDATE trips SET
		   name = COALESCE($2, name),
		   description = COALESCE($3, description),
		   start_date = COALESCE($4::date, start_date),
		   end_date = COALESCE($5::date, end_date),
		   timezone = COALESCE($6, timezone),
		   banner_url = CASE WHEN $7::bool THEN $8 ELSE banner_url END,
		   updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+access.TripColumns,
		id, data["name"], data["description"], data["start_date"], data["end_date"], data["timezone"],
		data["banner_set"] == true, data["banner_url"],
	)
	if err != nil {
		return nil, err
	}

	trip, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Trip])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("trip_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM schedule_blocks WHERE trip_id = $1`, id); err != nil {
		return err
	}
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
	return err
}

func (r *TripRepositoryImpl) ListMembers(ctx context.Context, tripID uuid.UUID) ([]entities.TripMember, error) {
	rows, err := r.db.Q(ctx).Query(ctx, memberSelect+` WHERE m.trip_id = $1 ORDER BY m.joined_at, u.name`, tripID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.TripMember])
}

func (r *TripRepositoryImpl) FindMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (*entities.TripMember, error) {
	rows, err := r.db.Q(ctx).Query(ctx, memberSelect+` WHERE m.trip_id = $1 AND m.user_id = $2`, tripID, userID)
	if err != nil {
		return nil, err
	}

	member, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.TripMember])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("member_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *TripRepositoryImpl) LockOwners(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT user_id FROM trip_members WHERE trip_id = $1 AND role = 'owner' FOR UPDATE`,
		tripID,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *TripRepositoryImpl) UpdateMemberRole(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE trip_members SET role = $3 WHERE trip_id = $1 AND user_id = $2`,
		tripID, userID, role,
	)
	return err
}

// RemoveMember also drops the member's calendar sync for the trip; their
// Google calendar is left alone, it is theirs.
func (r *TripRepositoryImpl) RemoveMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) error {
	if _, err := r.db.Q(ctx).Exec(ctx,
		`DELETE FROM calendar_syncs WHERE trip_id = $1 AND user_id = $2`, tripID, userID); err != nil {
		return err
	}
	_, err := r.db.Q(ctx).Exec(ctx,
		`DELETE FROM trip_members WHERE trip_id = $1 AND user_id = $2`, tripID, userID)
	return err
}

func (r *TripRepositoryImpl) BlocksOutside(ctx context.Context, tripID uuid.UUID, from time.Time, to time.Time) ([]uuid.UUID, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT id FROM schedule_blocks WHERE trip_id = $1 AND (start_at < $2 OR end_at > $3) ORDER BY start_at LIMIT 50`,
		tripID, from, to,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *TripRepositoryImpl) SyncStatus(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (dto.CalendarSyncStatus, error) {
	var status dto.CalendarSyncStatus
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT s.last_synced_at, s.last_error,
		        (SELECT COUNT(*) FROM sync_jobs j WHERE j.sync_id = s.id)::int
		 FROM calendar_syncs s WHERE s.trip_id = $1 AND s.user_id = $2`,
		tripID, userID,
	).Scan(&status.LastSyncedAt, &status.LastError, &status.PendingJobs)
	if errors.Is(err, pgx.ErrNoRows) {
		return dto.CalendarSyncStatus{}, nil
	}
	if err != nil {
		return dto.CalendarSyncStatus{}, err
	}

	status.Enabled = true
	return status, nil
}
