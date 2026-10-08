-- +goose Up
CREATE TABLE schedule_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    library_item_id UUID NULL,
    title TEXT NULL,
    note TEXT NOT NULL DEFAULT '',
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- RESTRICT: an item still used by the schedule cannot be deleted; the API
    -- offers ?force=true which removes the blocks first in one transaction.
    CONSTRAINT fk_schedule_blocks_library_item
        FOREIGN KEY (library_item_id, trip_id)
        REFERENCES library_items(id, trip_id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_schedule_blocks_range CHECK (end_at > start_at),
    CONSTRAINT chk_schedule_blocks_duration CHECK (end_at - start_at <= INTERVAL '24 hours'),
    CONSTRAINT chk_schedule_blocks_content CHECK (library_item_id IS NOT NULL OR title IS NOT NULL),

    -- One lane per trip: blocks may touch at the edges but never overlap, even
    -- when two requests race. Needs btree_gist for the uuid equality.
    CONSTRAINT excl_schedule_blocks_overlap
        EXCLUDE USING gist (trip_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
);

CREATE INDEX idx_schedule_blocks_trip_start ON schedule_blocks(trip_id, start_at);
CREATE INDEX idx_schedule_blocks_library_item ON schedule_blocks(library_item_id);

-- +goose Down
DROP TABLE IF EXISTS schedule_blocks;
