-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION auto_update_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS auto_update_at_column();
