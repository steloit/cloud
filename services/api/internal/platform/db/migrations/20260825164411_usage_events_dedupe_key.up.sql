-- O38: make a metering edge IDEMPOTENT, so retrying one is safe.
--
-- The point is not mainly that a retry double-bills today — MustEmitSpan does not
-- retry, it logs and continues. The point is that BECAUSE retrying is unsafe, we
-- do not retry, and a failed emit is therefore a silent BILLING GAP: the span
-- edge is lost, the invoice is wrong, and the recovery path is a human reading a
-- log line. A dedupe key inverts that — once a replay is safe, the emit can be
-- retried, and the gap closes.
--
-- The key is DERIVED by the caller from committed state, never random:
--   status transitions   <service_id>:<edge>:<services.status_changed_at>
--   reprice close/open   <service_id>:<edge>:reprice:<services.generation>
-- so two independent callers computing it for the same edge agree, and a genuinely
-- later transition computes a different one. A random key would satisfy the column
-- and nothing else.
--
-- Window: none. Every metering product bounds dedupe by time — Metronome 34 days,
-- OpenMeter 32, Stripe >=24h — because they hold events in a stream store and
-- cannot keep keys forever. We are in Postgres with a unique index, which is a
-- stronger guarantee than any window, and at this scale the index is free. If the
-- table is ever partitioned by period, the index becomes per-partition and the
-- window arrives on its own; that is the moment to revisit, not before.
ALTER TABLE usage_events ADD COLUMN dedupe_key text;

-- Backfill: rows written before this column exists cannot be reconstructed from a
-- derivation, because the state that identified their edge has since moved on.
-- Their own id is unique and is the honest key for them — it makes each existing
-- row its own dedupe class, which neither invents a history nor blocks the index.
UPDATE usage_events SET dedupe_key = id WHERE dedupe_key IS NULL;

ALTER TABLE usage_events ALTER COLUMN dedupe_key SET NOT NULL;

-- UNIQUE is the enforcement. Application-side checking would be a read-then-write
-- race, and this table is append-only precisely because billing truth must not
-- depend on the caller behaving.
CREATE UNIQUE INDEX usage_events_dedupe_key_idx ON usage_events (dedupe_key);

-- services.status_changed_at exists so a status transition HAS an identity.
-- Without it the tuple (service, edge, from->to) repeats every time a service
-- cycles ready -> suspended -> ready, and a dedupe key built from it would
-- collapse two genuine transitions into one — silently dropping revenue, which is
-- the opposite failure and a worse one.
ALTER TABLE services ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now();
