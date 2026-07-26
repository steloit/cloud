-- T7.4 / ADR-0007: org API keys carry an explicit permission subset (the
-- least-privilege scope) drawn from the canonical matrix. NULL for personal
-- tokens (they act as their user via membership); non-empty for org keys.
ALTER TABLE tokens ADD COLUMN permissions text[];
