BEGIN;

-- Controlled property type (mapping happens in parser)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'property_type') THEN
    CREATE TYPE property_type AS ENUM ('HOUSE', 'CONDO', 'TOWNHOUSE', 'DUPLEX', 'LAND', 'OTHER');
  END IF;
END$$;

-- Current snapshot per physical property identity
CREATE TABLE IF NOT EXISTS listings (
  property_key TEXT PRIMARY KEY,                   -- hash(normalized identity)
  property_type property_type NOT NULL,

  address TEXT NOT NULL,
  unit TEXT NULL,
  city TEXT NULL,
  province TEXT NULL,
  postal_code TEXT NOT NULL,

  lat DOUBLE PRECISION NULL,
  lon DOUBLE PRECISION NULL,

  beds INTEGER NULL,
  baths NUMERIC(3,1) NULL,
  sqft INTEGER NULL,

  current_price NUMERIC(12,2) NULL,
  url TEXT NULL,

  source TEXT NOT NULL DEFAULT 'REALTOR_CA',
  source_listing_id TEXT NULL,                     -- may be used as fallback for condos when unit missing

  first_seen_at TIMESTAMPTZ NOT NULL,
  last_seen_at  TIMESTAMPTZ NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Price and key attribute history (append-only)
CREATE TABLE IF NOT EXISTS price_history (
  id BIGSERIAL PRIMARY KEY,
  property_key TEXT NOT NULL REFERENCES listings(property_key) ON DELETE CASCADE,

  observed_at TIMESTAMPTZ NOT NULL,                -- scraped_at
  price NUMERIC(12,2) NOT NULL,

  beds INTEGER NULL,
  baths NUMERIC(3,1) NULL,
  sqft INTEGER NULL,
  url TEXT NULL,
  source_listing_id TEXT NULL,

  -- Prevent duplicates when a file is reprocessed
  CONSTRAINT price_history_unique_observation UNIQUE (property_key, observed_at, price)
);

-- Track processed raw files to guarantee idempotent ingestion
CREATE TABLE IF NOT EXISTS processed_files (
  id BIGSERIAL PRIMARY KEY,

  s3_bucket TEXT NOT NULL,
  s3_key TEXT NOT NULL,                            -- raw/realtorca/YYYY-MM-DD/run-*.jsonl

  etag TEXT NULL,
  size_bytes BIGINT NULL,

  status TEXT NOT NULL DEFAULT 'PROCESSED',         -- PROCESSED | FAILED
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- Vault references (for audit / restore)
  vault_raw_key TEXT NULL,
  vault_normalized_key TEXT NULL,
  vault_errors_key TEXT NULL,

  error_message TEXT NULL,

  CONSTRAINT processed_files_unique_object UNIQUE (s3_bucket, s3_key)
);

-- Useful indexes for future analytics/search
CREATE INDEX IF NOT EXISTS idx_listings_postal_code ON listings(postal_code);
CREATE INDEX IF NOT EXISTS idx_listings_property_type ON listings(property_type);
CREATE INDEX IF NOT EXISTS idx_listings_last_seen_at ON listings(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_price_history_property_time ON price_history(property_key, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_processed_files_processed_at ON processed_files(processed_at DESC);

COMMIT;
