-- +goose Up
CREATE TABLE release_notes (
    id BIGSERIAL PRIMARY KEY,
    version VARCHAR(50) NOT NULL,
    category SMALLINT NOT NULL,
    title VARCHAR(150) NOT NULL,
    parent_id BIGINT NULL,
    notes TEXT NOT NULL,
    release_date DATE NOT NULL,
    visibility SMALLINT NOT NULL,
    is_active SMALLINT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    created_by BIGINT NULL,
    updated_by BIGINT NULL,
    deleted_by BIGINT NULL,

    CONSTRAINT fk_release_notes_parent
        FOREIGN KEY (parent_id)
        REFERENCES release_notes(id)
);

CREATE INDEX idx_release_notes_version ON release_notes(version);
CREATE INDEX idx_release_notes_category ON release_notes(category);
CREATE INDEX idx_release_notes_parent_id ON release_notes(parent_id);
CREATE INDEX idx_release_notes_release_date ON release_notes(release_date);
CREATE INDEX idx_release_notes_visibility ON release_notes(visibility);
CREATE INDEX idx_release_notes_deleted_at ON release_notes(deleted_at);

-- +goose Down
DROP TABLE IF EXISTS release_notes;
