-- +goose Up
CREATE TABLE account (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(40) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1,
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT account_user_id_unique UNIQUE (user_id)
);

CREATE INDEX account_status_idx ON account (status);
CREATE INDEX account_version_idx ON account (version);

CREATE TRIGGER update_account_updated_at
    BEFORE UPDATE ON account
    FOR EACH ROW
    EXECUTE FUNCTION auto_update_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_account_updated_at ON account;
DROP INDEX IF EXISTS account_version_idx;
DROP INDEX IF EXISTS account_status_idx;
DROP TABLE IF EXISTS account;
