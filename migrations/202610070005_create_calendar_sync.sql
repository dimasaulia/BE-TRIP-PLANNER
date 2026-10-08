-- +goose Up
CREATE TABLE calendar_connections (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    google_email TEXT NOT NULL,
    refresh_token_enc BYTEA NOT NULL,
    scopes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_calendar_connections_status CHECK (status IN ('active', 'revoked'))
);

CREATE TABLE calendar_syncs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    google_calendar_id TEXT NOT NULL,
    last_synced_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_calendar_syncs_trip_user UNIQUE (trip_id, user_id)
);

CREATE INDEX idx_calendar_syncs_user_id ON calendar_syncs(user_id);

-- block_id has no foreign key so a deleted block can still be removed from Google.
CREATE TABLE calendar_event_links (
    sync_id UUID NOT NULL REFERENCES calendar_syncs(id) ON DELETE CASCADE,
    block_id UUID NOT NULL,
    google_event_id TEXT NOT NULL,
    content_hash TEXT NOT NULL,

    PRIMARY KEY (sync_id, block_id)
);

-- A pending job with the same (sync_id, block_id, op) is merged, so repeated
-- drags collapse into one delivery. full_resync uses the nil UUID as block_id.
CREATE TABLE sync_jobs (
    id BIGSERIAL PRIMARY KEY,
    sync_id UUID NOT NULL REFERENCES calendar_syncs(id) ON DELETE CASCADE,
    block_id UUID NOT NULL,
    op TEXT NOT NULL,
    run_after TIMESTAMPTZ NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    seq BIGINT NOT NULL DEFAULT 1,
    locked_until TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_sync_jobs_key UNIQUE (sync_id, block_id, op),
    CONSTRAINT chk_sync_jobs_op CHECK (op IN ('upsert', 'delete', 'full_resync'))
);

CREATE INDEX idx_sync_jobs_run_after ON sync_jobs(run_after);

-- +goose Down
DROP TABLE IF EXISTS sync_jobs;
DROP TABLE IF EXISTS calendar_event_links;
DROP TABLE IF EXISTS calendar_syncs;
DROP TABLE IF EXISTS calendar_connections;
