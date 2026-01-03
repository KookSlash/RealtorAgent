BEGIN;

CREATE TABLE IF NOT EXISTS processing_attempts (
  id BIGSERIAL PRIMARY KEY,

  s3_bucket TEXT NOT NULL,
  s3_key TEXT NOT NULL,
  etag TEXT NOT NULL,

  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT processing_attempts_unique_object
    UNIQUE (s3_bucket, s3_key, etag)
);

CREATE INDEX IF NOT EXISTS idx_processing_attempts_last_attempt
  ON processing_attempts(last_attempt_at DESC);

COMMIT;
