-- US-3.8: the override-expiry sweep runs `SELECT … WHERE override IS NOT NULL`
-- every 5 minutes, in every API replica. Without this it is a full scan of
-- `services` on each tick, and the CASE predicate is not sargable.
--
-- Partial, so the index is proportional to PINNED rows rather than to the
-- fleet: manual pins are rare and short-lived by construction (24h), so this
-- stays tiny. The repo already indexes its other sweeper predicates the same
-- way (invites_expires_idx, sessions_expires_at_idx).
CREATE INDEX IF NOT EXISTS services_override_idx ON services (id) WHERE override IS NOT NULL;
