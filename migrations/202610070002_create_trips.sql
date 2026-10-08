-- +goose Up
CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    timezone TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_trips_date_range CHECK (end_date >= start_date)
);

CREATE TABLE trip_members (
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (trip_id, user_id),
    CONSTRAINT chk_trip_members_role CHECK (role IN ('owner', 'editor', 'viewer'))
);

CREATE INDEX idx_trip_members_user_id ON trip_members(user_id);

-- Only the SHA-256 of the invite token is stored.
CREATE TABLE trip_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    role TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    max_uses INT NULL,
    use_count INT NOT NULL DEFAULT 0,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_trip_invites_token_hash UNIQUE (token_hash),
    CONSTRAINT chk_trip_invites_role CHECK (role IN ('editor', 'viewer')),
    CONSTRAINT chk_trip_invites_max_uses CHECK (max_uses IS NULL OR max_uses > 0)
);

CREATE INDEX idx_trip_invites_trip_id ON trip_invites(trip_id);

-- +goose Down
DROP TABLE IF EXISTS trip_invites;
DROP TABLE IF EXISTS trip_members;
DROP TABLE IF EXISTS trips;
