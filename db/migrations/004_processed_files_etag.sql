BEGIN;

-- Ensure etag exists
ALTER TABLE processed_files
  ADD COLUMN IF NOT EXISTS etag TEXT;

-- Drop old uniqueness if present
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'processed_files_unique_object'
  ) THEN
    ALTER TABLE processed_files DROP CONSTRAINT processed_files_unique_object;
  END IF;
END$$;

-- New uniqueness: same key can be reprocessed if content changed
ALTER TABLE processed_files
  ADD CONSTRAINT processed_files_unique_object
  UNIQUE (s3_bucket, s3_key, etag);

CREATE INDEX IF NOT EXISTS idx_processed_files_object
  ON processed_files(s3_bucket, s3_key);

COMMIT;
