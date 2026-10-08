package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
)

// BlockData is everything needed to build one Google event.
type BlockData struct {
	ID            uuid.UUID `db:"id"`
	TripID        uuid.UUID `db:"trip_id"`
	Title         *string   `db:"title"`
	Note          string    `db:"note"`
	StartAt       time.Time `db:"start_at"`
	EndAt         time.Time `db:"end_at"`
	ItemTitle     *string   `db:"item_title"`
	LocationName  *string   `db:"location_name"`
	DescriptionMD *string   `db:"description_md"`
	TripName      string    `db:"trip_name"`
	Timezone      string    `db:"timezone"`
}

type SyncCounts struct {
	LastSyncedAt *time.Time
	LastError    *string
	PendingJobs  int
}

type CalendarRepository interface {
	// Connection
	UpsertConnection(ctx context.Context, userID uuid.UUID, googleEmail string, refreshTokenEnc []byte, scopes string) error
	FindConnection(ctx context.Context, userID uuid.UUID) (*entities.CalendarConnection, error)
	MarkRevoked(ctx context.Context, userID uuid.UUID, reason string) error
	DeleteConnection(ctx context.Context, userID uuid.UUID) error

	// Syncs
	FindSync(ctx context.Context, tripID uuid.UUID, userID uuid.UUID) (*entities.CalendarSync, error)
	FindSyncByID(ctx context.Context, id uuid.UUID) (*entities.CalendarSync, error)
	CreateSync(ctx context.Context, tripID uuid.UUID, userID uuid.UUID, googleCalendarID string) (*entities.CalendarSync, error)
	DeleteSync(ctx context.Context, id uuid.UUID) error
	SetSyncResult(ctx context.Context, id uuid.UUID, syncedAt *time.Time, lastError *string) error
	PendingJobs(ctx context.Context, syncID uuid.UUID) (int, error)

	// Jobs
	EnqueueJob(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID, op string, delay time.Duration) error
	ClaimJobs(ctx context.Context, limit int) ([]entities.SyncJob, error)
	FinishJob(ctx context.Context, id int64, seq int64) error
	RetryJob(ctx context.Context, id int64, seq int64, attempts int, runAfter time.Time, lastError string) error

	// Event links and block data
	FindLink(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID) (*entities.CalendarEventLink, error)
	UpsertLink(ctx context.Context, link entities.CalendarEventLink) error
	DeleteLink(ctx context.Context, syncID uuid.UUID, blockID uuid.UUID) error
	ListLinkedBlockIDs(ctx context.Context, syncID uuid.UUID) ([]uuid.UUID, error)
	ListTripBlockIDs(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error)
	FindBlockData(ctx context.Context, blockID uuid.UUID) (*BlockData, error)
}
