-- 0002 — operation history, the options used, and sealed captured logs
-- (§7.7, ADR-0007/0009). Operation rows are immutable history; a re-run is a
-- new row. Sensitive captured logs are sealed before insert (ADR-0009): the
-- table stores only the AEAD nonce and ciphertext plus integrity metadata.

CREATE TABLE operations (
    id             TEXT    PRIMARY KEY,
    kind           TEXT    NOT NULL,
    src            TEXT    NOT NULL,
    dst            TEXT    NOT NULL,
    rclone_version TEXT    NOT NULL,
    intensity      TEXT    NOT NULL,
    started_at     TEXT    NOT NULL,
    ended_at       TEXT,
    bytes_moved    INTEGER NOT NULL DEFAULT 0,
    files_moved    INTEGER NOT NULL DEFAULT 0,
    result         TEXT    NOT NULL,
    log_blob_id    TEXT
);

CREATE INDEX idx_operations_started_at ON operations (started_at);
CREATE INDEX idx_operations_kind ON operations (kind);

CREATE TABLE operation_options (
    operation_id TEXT    NOT NULL REFERENCES operations (id) ON DELETE CASCADE,
    flag         TEXT    NOT NULL,
    value        TEXT    NOT NULL,
    risk         TEXT    NOT NULL,
    acknowledged INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_operation_options_op ON operation_options (operation_id);

CREATE TABLE log_blobs (
    id               TEXT    PRIMARY KEY,
    operation_id     TEXT    NOT NULL REFERENCES operations (id) ON DELETE CASCADE,
    nonce            BLOB    NOT NULL,
    sealed_bytes     BLOB    NOT NULL,
    sha256_plaintext TEXT    NOT NULL,
    bytes_len        INTEGER NOT NULL
);

CREATE INDEX idx_log_blobs_op ON log_blobs (operation_id);
