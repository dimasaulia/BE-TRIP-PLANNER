package entities

import (
	"time"

	"github.com/google/uuid"
)

// Trip dates are selected as `YYYY-MM-DD` text and mean calendar days in Timezone.
type Trip struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	StartDate   string    `db:"start_date" json:"start_date"`
	EndDate     string    `db:"end_date" json:"end_date"`
	Timezone    string    `db:"timezone" json:"timezone"`
	BannerURL   *string   `db:"banner_url" json:"banner_url"`
	CreatedBy   uuid.UUID `db:"created_by" json:"created_by"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Location resolves the IANA timezone; it was validated when the trip was saved.
func (t Trip) Location() *time.Location {
	location, err := time.LoadLocation(t.Timezone)
	if err != nil {
		return time.UTC
	}
	return location
}

type TripMember struct {
	TripID    uuid.UUID `db:"trip_id" json:"trip_id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Role      string    `db:"role" json:"role"`
	JoinedAt  time.Time `db:"joined_at" json:"joined_at"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	AvatarURL *string   `db:"avatar_url" json:"avatar_url"`
}

type TripInvite struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	TripID        uuid.UUID  `db:"trip_id" json:"trip_id"`
	Role          string     `db:"role" json:"role"`
	CreatedBy     uuid.UUID  `db:"created_by" json:"created_by"`
	CreatedByName string     `db:"created_by_name" json:"created_by_name"`
	ExpiresAt     time.Time  `db:"expires_at" json:"expires_at"`
	MaxUses       *int       `db:"max_uses" json:"max_uses"`
	UseCount      int        `db:"use_count" json:"use_count"`
	RevokedAt     *time.Time `db:"revoked_at" json:"revoked_at"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	TripName      string     `db:"trip_name" json:"-"`
}
