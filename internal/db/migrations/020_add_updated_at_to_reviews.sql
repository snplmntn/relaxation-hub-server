-- Migration 020: Add updated_at column to reviews table
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Create trigger to automatically update updated_at if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_reviews_updated_at') THEN
        CREATE TRIGGER update_reviews_updated_at
            BEFORE UPDATE ON reviews
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;
