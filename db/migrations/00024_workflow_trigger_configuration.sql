-- +goose Up
-- Declaration-only no-op for the requirement-first schema RED run.
SELECT 1;

-- +goose Down
SELECT 1;
