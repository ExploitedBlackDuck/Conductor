-- 0003 — saved sync/bisync pairs, named option profiles, and per-remote
-- governance ceilings (§7.5–7.7, ADR-0013). last_run_at distinguishes a pair's
-- first run, which defaults to dry-run (§7.4).

CREATE TABLE profiles (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL
);

CREATE TABLE profile_options (
    profile_id TEXT NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    flag       TEXT NOT NULL,
    value      TEXT NOT NULL
);

CREATE INDEX idx_profile_options_profile ON profile_options (profile_id);

CREATE TABLE saved_pairs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    path1       TEXT NOT NULL,
    path2       TEXT NOT NULL,
    profile_id  TEXT REFERENCES profiles (id) ON DELETE SET NULL,
    last_run_at TEXT
);

CREATE TABLE remote_ceilings (
    remote    TEXT PRIMARY KEY,
    transfers INTEGER NOT NULL DEFAULT 0,
    checkers  INTEGER NOT NULL DEFAULT 0,
    bwlimit   TEXT    NOT NULL DEFAULT '',
    tpslimit  INTEGER NOT NULL DEFAULT 0
);
