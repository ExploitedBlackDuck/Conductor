-- 0001 — the append-only, hash-chained audit log (ADR-0010, §7.8).
--
-- seq is the explicit 1-based chain position assigned by the audit service (not
-- an autoincrement), because the chain hash is computed over it before insert.
-- Rows are immutable once written; there is no UPDATE or DELETE path in the
-- store. detail holds canonical compact JSON; prev_hash is "" for the genesis
-- entry.
CREATE TABLE audit_log (
    seq       INTEGER PRIMARY KEY,
    at        TEXT    NOT NULL,
    action    TEXT    NOT NULL,
    subject   TEXT    NOT NULL,
    detail    TEXT    NOT NULL,
    prev_hash TEXT    NOT NULL,
    hash      TEXT    NOT NULL
);
