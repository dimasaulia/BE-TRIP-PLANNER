// Package syncqueue lets any module enqueue Google Calendar work inside its own
// transaction, so a job exists exactly when the change it describes committed.
package syncqueue

import (
	"context"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/entities"
	"github.com/open-suite/boilerplate-golang/internal/platform/database"
)

// Debounce is how long a job waits so a burst of edits becomes one delivery.
const Debounce = "5 seconds"

// FullResyncBlockID is the placeholder block id of full_resync jobs.
var FullResyncBlockID = uuid.Nil

type Queue interface {
	// EnqueueBlock queues one block for every calendar sync of the trip.
	EnqueueBlock(ctx context.Context, tripID uuid.UUID, blockID uuid.UUID, op string) error
	// EnqueueItemBlocks re-queues every block that shows the library item.
	EnqueueItemBlocks(ctx context.Context, itemID uuid.UUID) error
	// EnqueueTripResync queues a full resync for every sync of the trip.
	EnqueueTripResync(ctx context.Context, tripID uuid.UUID) error
}

type QueueImpl struct {
	db *database.Database
}

func NewQueue(db *database.Database) Queue {
	return &QueueImpl{db: db}
}

// A repeated key bumps seq and pushes run_after, which is the merge the TRD
// asks for; the worker uses seq to notice a job changed while it was running.
const upsertSuffix = `
ON CONFLICT (sync_id, block_id, op) DO UPDATE
SET run_after = EXCLUDED.run_after,
    seq = sync_jobs.seq + 1,
    attempts = 0,
    locked_until = NULL,
    last_error = NULL`

func (q *QueueImpl) EnqueueBlock(ctx context.Context, tripID uuid.UUID, blockID uuid.UUID, op string) error {
	_, err := q.db.Q(ctx).Exec(ctx,
		`INSERT INTO sync_jobs (sync_id, block_id, op, run_after)
		 SELECT id, $2, $3, NOW() + INTERVAL '`+Debounce+`' FROM calendar_syncs WHERE trip_id = $1`+upsertSuffix,
		tripID, blockID, op,
	)
	return err
}

func (q *QueueImpl) EnqueueItemBlocks(ctx context.Context, itemID uuid.UUID) error {
	_, err := q.db.Q(ctx).Exec(ctx,
		`INSERT INTO sync_jobs (sync_id, block_id, op, run_after)
		 SELECT s.id, b.id, $2, NOW() + INTERVAL '`+Debounce+`'
		 FROM schedule_blocks b
		 JOIN calendar_syncs s ON s.trip_id = b.trip_id
		 WHERE b.library_item_id = $1`+upsertSuffix,
		itemID, entities.SyncOpUpsert,
	)
	return err
}

func (q *QueueImpl) EnqueueTripResync(ctx context.Context, tripID uuid.UUID) error {
	_, err := q.db.Q(ctx).Exec(ctx,
		`INSERT INTO sync_jobs (sync_id, block_id, op, run_after)
		 SELECT id, $2, $3, NOW() FROM calendar_syncs WHERE trip_id = $1`+upsertSuffix,
		tripID, FullResyncBlockID, entities.SyncOpFullResync,
	)
	return err
}
