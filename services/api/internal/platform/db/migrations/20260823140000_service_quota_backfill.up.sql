-- US-3.3e: give every EXISTING service the envelope its org's plan grants.
--
-- WHY THIS IS A DEPLOY-STOP AND NOT A NICETY. tenancy.Render REFUSES a spec with
-- no envelope (it will not invent a ceiling nobody granted), and the renderer
-- calls it on every converge of every service. So the moment the new agent
-- rolls out, a service whose `desired` predates this feature does not merely run
-- unbounded — it fails to converge, forever, and the failure is a retry loop
-- rather than a status the customer can see. Every row must carry an envelope
-- before the agent that requires one is in front of it.
--
-- WHY THE VALUES ARE REPEATED HERE, AND WHAT KEEPS THEM HONEST. plans.json is
-- the ONE definition (AGENTS.md: a plan table in two modules is a plan table
-- that drifts), and SQL cannot read it. Rather than pretend, the duplication is
-- made *detectable*: TestTheBackfillMigrationMatchesThePlanTable parses this
-- VALUES list and fails if it differs from plans.Table.Envelope for any plan.
-- A migration is also immutable once shipped, so this list is a snapshot of
-- 2026-08-23 by design — the test asserts the snapshot was correct when written,
-- and a later plan change is US-3.3g's rewrite path, never an edit to this file.
--
-- Rows with `desired = '{}'` are skipped: they predate the reconciler columns
-- (20260721073228) and have no doc to extend — their doc is written whole on the
-- next touch, by desiredDoc, which supplies the envelope itself.
--
-- generation is bumped because the agent polls
-- `WHERE cell_id = $1 AND observed_generation < generation`; without the bump
-- the corrected doc is never fetched and the backfill is invisible.
UPDATE services s
SET desired    = s.desired || jsonb_build_object(
                     'quota',
                     jsonb_build_object('cpu', e.cpu, 'memory', e.memory, 'storage', e.storage)),
    generation = s.generation + 1
FROM environments env
    JOIN projects p ON p.id = env.project_id
    JOIN orgs o ON o.id = p.org_id
    JOIN (VALUES
              ('free', '1', '2Gi', '10Gi'),
              ('pro', '8', '16Gi', '100Gi'),
              ('business', '12', '24Gi', '200Gi'),
              ('enterprise', '16', '32Gi', '250Gi')
         ) AS e (plan, cpu, memory, storage) ON e.plan = o.plan
WHERE s.env_id = env.id
  AND s.desired <> '{}'::jsonb
  AND NOT (s.desired ? 'quota');
