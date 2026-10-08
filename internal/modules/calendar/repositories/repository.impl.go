package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
)

const (
	syncColumns       = "id, trip_id, user_id, google_calendar_id, last_synced_at, last_error, created_at"
	connectionColumns = "user_id, google_email, refresh_token_enc, scopes, status, created_at, updated_at"
)

type CalendarRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
}

func NewCalendarRepository(db *database.Database, appLogger *logger.Logger) CalendarRepository {
	return &CalendarRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.calendar"),
	}
}

func (r *CalendarRepositoryImpl) UpsertConnection(ctx context.Context, userID uuid.UUID, googleEmail string, refreshTokenEnc []byte, scopes string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO calendar_connections (user_id, google_email, refresh_token_enc, scopes, status)
		 VALUES ($1, $2, $3, $4, 'active')
		 ON CONFLICT (user_id) DO UPDATE
		 SET google_email = EXCLUDED.google_email, refresh_token_enc = EXCLUDED.refresh_token_enc,
		     scopes = EXCLUDED.scopes, status = 'active', updated_at = NOW()`,
		userID, googleEmail, refreshTokenEnc, scopes,
	)
	return err
}

func (r *CalendarRepositoryImpl) FindConnection(ctx context.Context, userID uuid.UUID) (*entities.CalendarConnection, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT `+connectionColumns+` FROM calendar_connections WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}

	connection, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.CalendarConnection])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// MarkRevoked flags the connection and stops all of the user's pending work.
func (r *CalendarRepositoryImpl) MarkRevoked(ctx context.Context, userID uuid.UUID, reason string) error {
	if _, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE calendar_connections SET status = 'revoked', updated_at = NOW() WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := r.db.Q(ctx).Exec(ctx,
		`DELETE FROM sync_jobs WHERE sync_id IN (SELECT id FROM calendar_syncs WHERE user_id = $1)`, userID); err != nil {
		return err
	}
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE calendar_syncs SET last_error = $2 WHERE user_id = $1`, userID, reason)
	return err
}

func (r *CalendarRepositoryImpl) DeleteConnection(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM calendar_syncs WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM calendar_connections WHERE user_id = $1`, userID)
	return err
}

func (r *CalendarRepositoryImpl) oneSync(ctx context.Context, query string, args ...any) (*entities.CalendarSync, error) {
	rows, err := r.db.Q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	sync, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.CalendarSync])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sync, nil
}

func (r *CalendarRepositoryImpl) FindSync(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (*entities.CalendarSync, error) {
	return r.oneSync(ctx, `SELECT `+syncColumns+` FROM calendar_syncs WHERE trip_id = $1 AND user_id = $2`, tripID, userID)
}

func (r *CalendarRepositoryImpl) FindSyncByID(ctx context.Context, id uuid.UUID) (*entities.CalendarSync, error) {
	return r.oneSync(ctx, `SELECT `+syncColumns+` FROM calendar_syncs WHERE id = $1`, id)
}

func (r *CalendarRepositoryImpl) CreateSync(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, googleCalendarID string) (*entities.CalendarSync, error) {
	return r.oneSync(ctx,
		`INSERT INTO calendar_syncs (trip_id, user_id, google_calendar_id) VALUES ($1, $2, $3) RETURNING `+syncColumns,
		tripID, userID, googleCalendarID,
	)
}

func (r *CalendarRepositoryImpl) DeleteSync(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM calendar_syncs WHERE id = $1`, id)
	return err
}

func (r *CalendarRepositoryImpl) SetSyncResult(ctx context.Context, id uuid.UUID, syncedAt *time.Time, lastError *string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE calendar_syncs SET last_synced_at = COALESCE($2, last_synced_at), last_error = $3 WHERE id = $1`,
		id, syncedAt, lastError,
	)
	return err
}

func (r *CalendarRepositoryImpl) PendingJobs(ctx context.Context, syncID uuid.UUID) (int, error) {
	var count int
	err := r.db.Q(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM sync_jobs WHERE sync_id = $1`, syncID).Scan(&count)
	return count, err
}

func (r *CalendarRepositoryImpl) EnqueueJob(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID, op string, delay time.Duration) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO sync_jobs (sync_id, block_id, op, run_after)
		 VALUES ($1, $2, $3, NOW() + make_interval(secs => $4))
		 ON CONFLICT (sync_id, block_id, op) DO UPDATE
		 SET run_after = EXCLUDED.run_after, seq = sync_jobs.seq + 1, attempts = 0, locked_until = NULL, last_error = NULL`,
		syncID, blockID, op, delay.Seconds(),
	)
	return err
}

// ClaimJobs leases due jobs with FOR UPDATE SKIP LOCKED so several workers
// never take the same one; the lease is short and the work happens afterwards.
func (r *CalendarRepositoryImpl) ClaimJobs(ctx context.Context, limit int) ([]entities.SyncJob, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`UPDATE sync_jobs SET locked_until = NOW() + INTERVAL '2 minutes'
		 WHERE id IN (
		     SELECT id FROM sync_jobs
		     WHERE run_after <= NOW() AND (locked_until IS NULL OR locked_until < NOW())
		     ORDER BY run_after, id
		     LIMIT $1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, sync_id, block_id, op, attempts, seq`,
		limit,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.SyncJob])
}

// FinishJob deletes the job unless it was re-enqueued (seq moved) while running.
func (r *CalendarRepositoryImpl) FinishJob(ctx context.Context, id int64, seq int64) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM sync_jobs WHERE id = $1 AND seq = $2`, id, seq)
	return err
}

func (r *CalendarRepositoryImpl) RetryJob(ctx context.Context, id int64, seq int64, attempts int, runAfter time.Time, lastError string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE sync_jobs SET attempts = $3, run_after = $4, last_error = $5, locked_until = NULL
		 WHERE id = $1 AND seq = $2`,
		id, seq, attempts, runAfter, lastError,
	)
	return err
}

func (r *CalendarRepositoryImpl) FindLink(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID) (*entities.CalendarEventLink, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT sync_id, block_id, google_event_id, content_hash FROM calendar_event_links WHERE sync_id = $1 AND block_id = $2`,
		syncID, blockID,
	)
	if err != nil {
		return nil, err
	}

	link, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.CalendarEventLink])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *CalendarRepositoryImpl) UpsertLink(ctx context.Context, link entities.CalendarEventLink) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO calendar_event_links (sync_id, block_id, google_event_id, content_hash)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (sync_id, block_id) DO UPDATE
		 SET google_event_id = EXCLUDED.google_event_id, content_hash = EXCLUDED.content_hash`,
		link.SyncID, link.BlockID, link.GoogleEventID, link.ContentHash,
	)
	return err
}

func (r *CalendarRepositoryImpl) DeleteLink(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM calendar_event_links WHERE sync_id = $1 AND block_id = $2`, syncID, blockID)
	return err
}

func (r *CalendarRepositoryImpl) ListLinkedBlockIDs(ctx context.Context, syncID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT block_id FROM calendar_event_links WHERE sync_id = $1`, syncID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *CalendarRepositoryImpl) ListTripBlockIDs(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT id FROM schedule_blocks WHERE trip_id = $1`, tripID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *CalendarRepositoryImpl) FindBlockData(ctx context.Context, blockID uuid.UUID) (*BlockData, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT b.id, b.trip_id, b.title, b.note, b.start_at, b.end_at,
		        i.title AS item_title, i.location_name, i.description_md,
		        t.name AS trip_name, t.timezone
		 FROM schedule_blocks b
		 LEFT JOIN library_items i ON i.id = b.library_item_id
		 JOIN trips t ON t.id = b.trip_id
		 WHERE b.id = $1`,
		blockID,
	)
	if err != nil {
		return nil, err
	}

	data, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[BlockData])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &data, nil
}
