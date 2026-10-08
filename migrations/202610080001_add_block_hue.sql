-- +goose Up
-- Colour of a note made directly on the timeline (no library item). Stored so
-- every member sees the same colour; blocks of a library item follow its category.
ALTER TABLE schedule_blocks
    ADD COLUMN hue INT NULL,
    ADD CONSTRAINT chk_schedule_blocks_hue CHECK (hue IS NULL OR hue BETWEEN 0 AND 359);

-- +goose Down
ALTER TABLE schedule_blocks
    DROP CONSTRAINT IF EXISTS chk_schedule_blocks_hue,
    DROP COLUMN IF EXISTS hue;
