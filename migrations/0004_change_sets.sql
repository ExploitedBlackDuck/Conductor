-- 0004 — sealed dry-run change sets (ADR-0015, §7.7). The destructive-op preview
-- the operator was shown and confirmed against, retained as tamper-evident
-- evidence of what was acknowledged. The counts are queryable in the clear; the
-- path lists are as sensitive as captured logs (they may name real files) and
-- are AEAD-sealed before insert (ADR-0009) — the table never holds them in the
-- clear. One change set per operation.

CREATE TABLE change_sets (
    operation_id     TEXT    PRIMARY KEY REFERENCES operations (id) ON DELETE CASCADE,
    create_count     INTEGER NOT NULL DEFAULT 0,
    update_count     INTEGER NOT NULL DEFAULT 0,
    delete_count     INTEGER NOT NULL DEFAULT 0,
    truncated        INTEGER NOT NULL DEFAULT 0,
    acknowledged_at  TEXT    NOT NULL,
    nonce            BLOB    NOT NULL,
    sealed_bytes     BLOB    NOT NULL,
    sha256_plaintext TEXT    NOT NULL,
    bytes_len        INTEGER NOT NULL
);
