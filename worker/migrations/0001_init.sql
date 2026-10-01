-- Links are authoritative. Timestamps are unix seconds (UTC).
CREATE TABLE links (
  short_code    TEXT PRIMARY KEY,
  long_url      TEXT NOT NULL,
  owner_id      TEXT NOT NULL DEFAULT '',
  is_custom     INTEGER NOT NULL,
  is_active     INTEGER NOT NULL DEFAULT 1,
  version       INTEGER NOT NULL DEFAULT 1,
  created_at    INTEGER NOT NULL,
  expires_at    INTEGER NOT NULL,
  deleted_at    INTEGER,
  purge_at      INTEGER NOT NULL,
  deleted_by    TEXT NOT NULL DEFAULT '',
  delete_role   TEXT NOT NULL DEFAULT '',
  delete_reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX links_purge_at ON links (purge_at);

-- Idempotency replay records, keyed by owner + client key, expire after 24h.
CREATE TABLE idempotency (
  owner_key    TEXT PRIMARY KEY,
  short_code   TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  ttl          INTEGER NOT NULL
);
CREATE INDEX idempotency_ttl ON idempotency (ttl);

-- Block-leased monotonic counter; one row per allocator region.
CREATE TABLE counters (
  region  TEXT PRIMARY KEY,
  counter INTEGER NOT NULL DEFAULT 0
);
INSERT INTO counters (region, counter) VALUES ('global', 0);

-- Click counters, incremented off the redirect response path.
CREATE TABLE stats (
  short_code    TEXT PRIMARY KEY,
  clicks        INTEGER NOT NULL DEFAULT 0,
  last_click_at INTEGER NOT NULL DEFAULT 0
);

-- Operator takedown audit trail.
CREATE TABLE audit (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_id   TEXT NOT NULL,
  role       TEXT NOT NULL,
  owner_id   TEXT NOT NULL,
  short_code TEXT NOT NULL,
  reason     TEXT NOT NULL,
  outcome    TEXT NOT NULL,
  ts         INTEGER NOT NULL
);
