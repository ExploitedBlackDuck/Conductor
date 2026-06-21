-- 0005 — signed audit-chain anchors (ADR-0010, §7.8). Hash-chaining detects
-- partial/naive edits; an anchor signs a chain head with a separate keyring-held
-- key so a full recompute of the chain is detectable without that key. Anchors
-- are append-only like the log itself; the newest (highest seq) is the current
-- signed head. The signature is a MAC, not secret on its own.

CREATE TABLE audit_anchors (
    seq       INTEGER NOT NULL,
    head_hash TEXT    NOT NULL,
    signature BLOB    NOT NULL,
    signed_at TEXT    NOT NULL,
    PRIMARY KEY (seq, signed_at)
);

CREATE INDEX idx_audit_anchors_seq ON audit_anchors (seq);
