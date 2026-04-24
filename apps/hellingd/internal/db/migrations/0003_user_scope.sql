-- +goose Up
-- +goose StatementBegin

-- Add the Incus trust-scope flag to users. Nullable until the internal CA
-- lands and per-user certificates begin reading from this column. Valid
-- values are `default`/`restricted`/`admin` per docs/spec/auth.md §4 and
-- docs/decisions/024-incus-native-auth.md.
ALTER TABLE users ADD COLUMN scope TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN scope;

-- +goose StatementEnd
