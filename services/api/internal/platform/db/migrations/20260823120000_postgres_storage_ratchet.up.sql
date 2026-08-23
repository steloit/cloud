-- T3.4c: normalise every postgres service's stored `storage_gb` up to the
-- included amount for its size.
--
-- WHY THIS EXISTS. `storage_gb` in a persisted shape is the PROVISIONED
-- RATCHET, not a free-floating declaration: a PersistentVolumeClaim cannot
-- shrink (Kubernetes only supports expansion, and a request below
-- `.status.capacity` is rejected), so the volume a service has is a floor it
-- can never go below. Before T3.4c the API resolved an unset `storage_gb` to 0
-- while the driver rendered a size-derived volume, so rows recorded 0 while
-- running 32Gi or 10Gi. That split is what T3.4c closes at the gate; this
-- migration closes it for rows that already exist.
--
-- Without it the SAME configuration prices differently depending on when the
-- row was created: a `standard` created after T3.4c stores 50 and, on a later
-- downgrade to `dev`, prices 1900 + 50GB*50c = 4400; an older row stores 0 and
-- prices 1900 for an identical resulting configuration — while both keep a
-- volume they cannot give back.
--
-- IT IS PRICE-NEUTRAL AT REST, which is why it is a migration and not a
-- founder decision. Price charges `storage_gb - included_gb` only when
-- positive, so raising a stored value TO included_gb adds exactly zero:
--   dev          1900  -> 1900
--   standard     5800  -> 5800
--   performance 11200  -> 11200
-- Verified by TestTheStorageRatchetMigrationIsPriceNeutral, which drives the
-- real pricing engine over every catalog size rather than restating these.
--
-- The included amounts are embedded rather than read from
-- `services/api/internal/estimates/pricing.json`, because a migration is a
-- historical record of what was applied on this date. A test asserts they
-- matched the catalog when this was written; if the catalog changes later, that
-- is a new migration, not an edit to this one.
--
-- `desired` is updated in the same statement. It is the document the cell-agent
-- converges from, and leaving the two disagreeing is the exact
-- declared-vs-provisioned split above, one layer down.

-- THE CAST IS GUARDED. `(shape ->> 'storage_gb')::int` aborts the WHOLE
-- migration on any non-integer value, and engine.go:212-214 documents by name
-- that pre-US-3.7 rows persisted the raw shape verbatim — `{storage_gb: "78"}`
-- is a value this table is known to have held. Measured on postgres:16-alpine:
-- `"50Gi"` gives `ERROR: invalid input syntax for type integer`, golang-migrate
-- leaves schema_migrations dirty, and the deploy stops until someone forces it.
-- A migration cannot presuppose legacy rows exist AND presuppose they are all
-- well-typed.
--
-- pg_input_is_valid is PG16, which db.go already requires. A numeric STRING is
-- honoured ("78" means 78 GB); anything un-interpretable ("50Gi") reads as 0 and
-- is raised to included_gb, which is the safe floor and what the driver renders
-- anyway.

CREATE OR REPLACE FUNCTION pg_temp.storage_gb_of(v jsonb) RETURNS int
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN v IS NULL OR jsonb_typeof(v) = 'null' THEN 0
    WHEN jsonb_typeof(v) = 'number' THEN (v #>> '{}')::int
    WHEN jsonb_typeof(v) = 'string' AND pg_input_is_valid(v #>> '{}', 'integer')
      THEN (v #>> '{}')::int
    ELSE 0
  END
$$;

UPDATE services
SET
    shape = jsonb_set(
        shape, '{storage_gb}',
        to_jsonb(GREATEST(pg_temp.storage_gb_of(shape -> 'storage_gb'), v.included_gb)),
        true
    ),
    desired = CASE
        WHEN desired ? 'shape' THEN jsonb_set(
            desired, '{shape,storage_gb}',
            to_jsonb(GREATEST(pg_temp.storage_gb_of(desired -> 'shape' -> 'storage_gb'), v.included_gb)),
            true
        )
        ELSE desired
    END
FROM (VALUES
    ('dev', 0),
    ('standard', 50),
    ('performance', 50)
) AS v(size, included_gb)
WHERE services.product = 'postgres'
  AND services.shape ->> 'size' = v.size
  AND pg_temp.storage_gb_of(services.shape -> 'storage_gb') < v.included_gb;
