-- +goose Up
CREATE TABLE library_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    description_md TEXT NOT NULL DEFAULT '',
    location_name TEXT NULL,
    lat DOUBLE PRECISION NULL,
    lng DOUBLE PRECISION NULL,
    link_url TEXT NULL,
    fixed_start_at TIMESTAMPTZ NULL,
    fixed_end_at TIMESTAMPTZ NULL,
    version INT NOT NULL DEFAULT 1,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Lets schedule_blocks reference an item and its trip together.
    CONSTRAINT uq_library_items_id_trip UNIQUE (id, trip_id),
    CONSTRAINT chk_library_items_kind CHECK (kind IN ('destination', 'event')),
    CONSTRAINT chk_library_items_description_size CHECK (octet_length(description_md) <= 102400),
    CONSTRAINT chk_library_items_lat CHECK (lat IS NULL OR lat BETWEEN -90 AND 90),
    CONSTRAINT chk_library_items_lng CHECK (lng IS NULL OR lng BETWEEN -180 AND 180),
    CONSTRAINT chk_library_items_fixed CHECK (fixed_start_at IS NULL OR fixed_end_at IS NULL OR fixed_end_at >= fixed_start_at)
);

CREATE INDEX idx_library_items_trip_created ON library_items(trip_id, created_at DESC, id DESC);

CREATE TABLE attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    uploaded_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    stored_name TEXT NOT NULL,
    original_name TEXT NOT NULL,
    mime TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_attachments_stored_name UNIQUE (stored_name)
);

CREATE INDEX idx_attachments_trip_id ON attachments(trip_id);

-- +goose Down
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS library_items;
