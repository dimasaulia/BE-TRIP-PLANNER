-- +goose Up
-- Fields the workspace design needs: a free text category (its colour lives in
-- the browser only), a default duration used when the item is dropped on the
-- timeline, and a banner image like the trip banner.
ALTER TABLE library_items
    ADD COLUMN category TEXT NULL,
    ADD COLUMN default_duration_min INT NOT NULL DEFAULT 60,
    ADD COLUMN banner_url TEXT NULL,
    ADD CONSTRAINT chk_library_items_category CHECK (category IS NULL OR char_length(category) BETWEEN 1 AND 40),
    ADD CONSTRAINT chk_library_items_default_duration CHECK (default_duration_min BETWEEN 30 AND 240 AND default_duration_min % 5 = 0);

-- +goose Down
ALTER TABLE library_items
    DROP CONSTRAINT IF EXISTS chk_library_items_default_duration,
    DROP CONSTRAINT IF EXISTS chk_library_items_category,
    DROP COLUMN IF EXISTS banner_url,
    DROP COLUMN IF EXISTS default_duration_min,
    DROP COLUMN IF EXISTS category;
