-- +goose Up
ALTER TABLE trips ADD COLUMN banner_url TEXT NULL;

-- +goose Down
ALTER TABLE trips DROP COLUMN IF EXISTS banner_url;
