-- Secret blobs (envelope-encrypted ciphertext) live in Postgres itself:
-- they are opaque KV bytes, so a separate object store (MinIO — repo
-- archived 2026-04-25) added an extra service without adding capability.
CREATE TABLE secret_blobs (
    storage_key TEXT PRIMARY KEY,
    data        BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
