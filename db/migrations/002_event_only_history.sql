BEGIN;

ALTER TABLE price_history
  ADD COLUMN IF NOT EXISTS snapshot_hash TEXT;

-- Drop old uniqueness constraint (created in 001) if present
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'price_history_unique_observation'
  ) THEN
    ALTER TABLE price_history DROP CONSTRAINT price_history_unique_observation;
  END IF;
END$$;

-- Retry-safe uniqueness: same observation timestamp + same content hash
ALTER TABLE price_history
  ADD CONSTRAINT price_history_unique_observation
  UNIQUE (property_key, observed_at, snapshot_hash);

CREATE INDEX IF NOT EXISTS idx_price_history_hash
  ON price_history(property_key, snapshot_hash);

COMMIT;
