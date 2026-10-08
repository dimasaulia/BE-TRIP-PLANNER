// Package access answers "may this user do this in this trip?" for every
// module, so the 404-for-non-members / 403-for-low-role rule lives in one place.
package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
	"github.com/open-suite/boilerplate-golang/internal/shared/apperror"
)

type Role string

const (
	Owner  Role = "owner"
	Editor Role = "editor"
	Viewer Role = "viewer"
)

func (r Role) rank() int {
	switch r {
	case Owner:
		return 3
	case Editor:
		return 2
	case Viewer:
		return 1
	default:
		return 0
	}
}

func (r Role) AtLeast(min Role) bool {
	return r.rank() >= min.rank()
}

func (r Role) Valid() bool {
	return r.rank() > 0
}

type Membership struct {
	TripID uuid.UUID
	UserID uuid.UUID
	Role   Role
}

// TripColumns selects a trip with dates as YYYY-MM-DD text.
const TripColumns = "id, name, description, start_date::text AS start_date, end_date::text AS end_date, timezone, banner_url, created_by, created_at, updated_at"

type Service interface {
	// Require loads the membership and enforces a minimum role. Non members get
	// 404 trip_not_found so the existence of a trip does not leak.
	Require(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, min Role) (*Membership, error)
	Trip(ctx context.Context, tripID uuid.UUID) (*entities.Trip, error)
}

type ServiceImpl struct {
	db *database.Database
}

func NewService(db *database.Database) Service {
	return &ServiceImpl{db: db}
}

func (s *ServiceImpl) Require(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, min Role) (*Membership, error) {
	var role string
	err := s.db.Q(ctx).QueryRow(ctx,
		`SELECT role FROM trip_members WHERE trip_id = $1 AND user_id = $2`,
		tripID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("trip_not_found")
	}
	if err != nil {
		return nil, err
	}

	membership := &Membership{TripID: tripID, UserID: userID, Role: Role(role)}
	if !membership.Role.AtLeast(min) {
		return nil, apperror.Forbidden("forbidden").With("required_role", string(min))
	}

	return membership, nil
}

func (s *ServiceImpl) Trip(ctx context.Context, tripID uuid.UUID) (*entities.Trip, error) {
	rows, err := s.db.Q(ctx).Query(ctx, `SELECT `+TripColumns+` FROM trips WHERE id = $1`, tripID)
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

// RequireFor is Require for a resource addressed by its own id (library item,
// schedule block): a non member gets that resource's not-found code, so ids
// cannot be probed to learn which resources exist in trips they cannot see.
func RequireFor(ctx context.Context, service Service, tripID uuid.UUID, userID uuid.UUID, min Role, notFoundCode string) (*Membership, error) {
	membership, err := service.Require(ctx, tripID, userID, min)
	if appErr, ok := apperror.As(err); ok && appErr.Code == "trip_not_found" {
		return nil, apperror.NotFound(notFoundCode)
	}
	return membership, err
}
