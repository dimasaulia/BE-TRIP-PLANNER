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
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

const inviteSelect = `SELECT i.id, i.trip_id, i.role, i.created_by, u.name AS created_by_name,
	i.expires_at, i.max_uses, i.use_count, i.revoked_at, i.created_at, t.name AS trip_name
	FROM trip_invites i
	JOIN users u ON u.id = i.created_by
	JOIN trips t ON t.id = i.trip_id`

type InviteRepositoryImpl struct {
	db  *database.Database
	log *logger.LayerLogger
}

func NewInviteRepository(db *database.Database, appLogger *logger.Logger) InviteRepository {
	return &InviteRepositoryImpl{
		db:  db,
		log: appLogger.Layer("repository.invites"),
	}
}

func (r *InviteRepositoryImpl) one(ctx context.Context, query string, args ...any) (*entities.TripInvite, error) {
	rows, err := r.db.Q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	invite, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.TripInvite])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("invite_not_found")
	}
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *InviteRepositoryImpl) Create(ctx context.Context, tripID uuid.UUID, tokenHash []byte, role string, createdBy uuid.UUID, expiresAt time.Time, maxUses *int) (*entities.TripInvite, error) {
	var id uuid.UUID
	err := r.db.Q(ctx).QueryRow(ctx,
		`INSERT INTO trip_invites (trip_id, token_hash, role, created_by, expires_at, max_uses)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tripID, tokenHash, role, createdBy, expiresAt, maxUses,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	return r.one(ctx, inviteSelect+` WHERE i.id = $1`, id)
}

func (r *InviteRepositoryImpl) ListActive(ctx context.Context, tripID uuid.UUID) ([]entities.TripInvite, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		inviteSelect+` WHERE i.trip_id = $1 AND i.revoked_at IS NULL AND i.expires_at > NOW()
		 AND (i.max_uses IS NULL OR i.use_count < i.max_uses)
		 ORDER BY i.created_at DESC`,
		tripID,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.TripInvite])
}

func (r *InviteRepositoryImpl) FindByID(ctx context.Context, tripID uuid.UUID, inviteID uuid.UUID) (*entities.TripInvite, error) {
	return r.one(ctx, inviteSelect+` WHERE i.id = $1 AND i.trip_id = $2`, inviteID, tripID)
}

func (r *InviteRepositoryImpl) Revoke(ctx context.Context, inviteID uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE trip_invites SET revoked_at = COALESCE(revoked_at, NOW()) WHERE id = $1`, inviteID)
	return err
}

func (r *InviteRepositoryImpl) FindByTokenHash(ctx context.Context, tokenHash []byte, lock bool) (*entities.TripInvite, error) {
	query := inviteSelect + ` WHERE i.token_hash = $1`
	if lock {
		query += ` FOR UPDATE OF i`
	}
	return r.one(ctx, query, tokenHash)
}

func (r *InviteRepositoryImpl) IncrementUse(ctx context.Context, inviteID uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE trip_invites SET use_count = use_count + 1 WHERE id = $1`, inviteID)
	return err
}

func (r *InviteRepositoryImpl) MemberRole(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (string, bool, error) {
	var role string
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT role FROM trip_members WHERE trip_id = $1 AND user_id = $2`, tripID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return role, true, nil
}

func (r *InviteRepositoryImpl) AddMember(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, role string) (*entities.TripMember, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`WITH inserted AS (
		     INSERT INTO trip_members (trip_id, user_id, role) VALUES ($1, $2, $3)
		     RETURNING trip_id, user_id, role, joined_at
		 )
		 SELECT i.trip_id, i.user_id, i.role, i.joined_at, u.name, u.email, u.avatar_url
		 FROM inserted i JOIN users u ON u.id = i.user_id`,
		tripID, userID, role,
	)
	if err != nil {
		return nil, err
	}

	member, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.TripMember])
	if err != nil {
		return nil, err
	}
	return &member, nil
}
