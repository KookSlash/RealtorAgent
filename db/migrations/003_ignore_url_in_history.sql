BEGIN;

-- URL is stored on listings (current snapshot), but not used for event-only history.
ALTER TABLE price_history
  DROP COLUMN IF EXISTS url;

COMMIT;
