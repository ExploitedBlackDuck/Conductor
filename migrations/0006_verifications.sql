-- 0006 — integrity verifications (§7.7, §7.12). The recorded result of a
-- check/cryptcheck: source vs destination, with match/differ/missing/error
-- counts and a verdict. Read-only evidence — a verification never mutates a
-- remote — and is also hash-chained into the audit log (§7.8), so "this sync was
-- verified and matched" is a durable, tamper-evident claim. Only counts are
-- stored; the offending paths are shown live, not persisted.

CREATE TABLE verifications (
    id          TEXT    PRIMARY KEY,
    kind        TEXT    NOT NULL,
    src         TEXT    NOT NULL,
    dst         TEXT    NOT NULL,
    started_at  TEXT    NOT NULL,
    ended_at    TEXT,
    match_count INTEGER NOT NULL DEFAULT 0,
    differ      INTEGER NOT NULL DEFAULT 0,
    missing     INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    result      TEXT    NOT NULL
);

CREATE INDEX idx_verifications_started_at ON verifications (started_at);
