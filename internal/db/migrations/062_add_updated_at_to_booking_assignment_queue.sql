-- +goose Up
ALTER TABLE booking_assignment_queue ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
DROP TRIGGER IF EXISTS update_booking_assignment_queue_updated_at ON booking_assignment_queue;
CREATE TRIGGER update_booking_assignment_queue_updated_at
    BEFORE UPDATE ON booking_assignment_queue
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_booking_assignment_queue_updated_at ON booking_assignment_queue;
ALTER TABLE booking_assignment_queue DROP COLUMN IF EXISTS updated_at;
