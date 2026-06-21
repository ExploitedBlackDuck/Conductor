-- 0007 — record server-side eligibility on operations (§7.3, §7.7). When source
-- and destination share a backend identity, rclone performs the copy/move
-- server-side and data is not proxied through the operator's link; Conductor
-- records the expectation it detected. Existing rows default to 0 (not
-- server-side), which is correct — they were not flagged.

ALTER TABLE operations ADD COLUMN server_side INTEGER NOT NULL DEFAULT 0;
