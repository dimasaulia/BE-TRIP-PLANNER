package entities

import (
	"time"

	"github.com/google/uuid"
)

const (
	CalendarStatusActive  = "active"
	CalendarStatusRevoked = "revoked"
)

type CalendarConnection struct {
	UserID          uuid.UUID `db:"user_id" json:"user_id"`
	GoogleEmail     string    `db:"google_email" json:"google_email"`
	RefreshTokenEnc []byte    `db:"refresh_token_enc" json:"-"`
	Scopes          string    `db:"scopes" json:"scopes"`
	Status          string    `db:"status" json:"status"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

type CalendarSync struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	TripID           uuid.UUID  `db:"trip_id" json:"trip_id"`
	UserID           uuid.UUID  `db:"user_id" json:"user_id"`
	GoogleCalendarID string     `db:"google_calendar_id" json:"google_calendar_id"`
	LastSyncedAt     *time.Time `db:"last_synced_at" json:"last_synced_at"`
	LastError        *string    `db:"last_error" json:"last_error"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
}

type CalendarEventLink struct {
	SyncID        uuid.UUID `db:"sync_id"`
	BlockID       uuid.UUID `db:"block_id"`
	GoogleEventID string    `db:"google_event_id"`
	ContentHash   string    `db:"content_hash"`
}

const (
	SyncOpUpsert     = "upsert"
	SyncOpDelete     = "delete"
	SyncOpFullResync = "full_resync"
)

type SyncJob struct {
	ID       int64     `db:"id"`
	SyncID   uuid.UUID `db:"sync_id"`
	BlockID  uuid.UUID `db:"block_id"`
	Op       string    `db:"op"`
	Attempts int       `db:"attempts"`
	Seq      int64     `db:"seq"`
}
