package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/platform/logger"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const userColumns = "id, google_sub, email, name, avatar_url, created_at, updated_at"

type AuthRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
}

func NewAuthRepository(db *database.Database, appLogger *logger.Logger) AuthRepository {
	return &AuthRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.auth"),
	}
}

func (r *AuthRepositoryImpl) UpsertUser(ctx context.Context, googleSub string, email string, name string, avatarURL *string) (*entities.User, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`INSERT INTO users (google_sub, email, name, avatar_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (google_sub) DO UPDATE
		 SET email = EXCLUDED.email, name = EXCLUDED.name, avatar_url = EXCLUDED.avatar_url, updated_at = NOW()
		 RETURNING `+userColumns,
		googleSub, email, name, avatarURL,
	)
	if err != nil {
		return nil, err
	}

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apperror.Conflict("account_conflict")
		}
		return nil, err
	}

	return &user, nil
}

func (r *AuthRepositoryImpl) FindUser(ctx context.Context, id uuid.UUID) (*entities.User, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Unauthorized()
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *AuthRepositoryImpl) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (r *AuthRepositoryImpl) FindSession(ctx context.Context, tokenHash []byte) (*SessionRecord, error) {
	var record SessionRecord
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT s.id, s.expires_at, u.id, u.google_sub, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > NOW()`,
		tokenHash,
	).Scan(
		&record.SessionID, &record.ExpiresAt,
		&record.User.ID, &record.User.GoogleSub, &record.User.Email, &record.User.Name,
		&record.User.AvatarURL, &record.User.CreatedAt, &record.User.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Unauthorized()
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (r *AuthRepositoryImpl) ExtendSession(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE sessions SET expires_at = $2 WHERE id = $1`, sessionID, expiresAt)
	return err
}

func (r *AuthRepositoryImpl) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *AuthRepositoryImpl) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := r.db.Q(ctx).Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *AuthRepositoryImpl) FindCalendarConnection(ctx context.Context, userID uuid.UUID) (*entities.CalendarConnection, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT user_id, google_email, refresh_token_enc, scopes, status, created_at, updated_at
		 FROM calendar_connections WHERE user_id = $1`,
		userID,
	)
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
