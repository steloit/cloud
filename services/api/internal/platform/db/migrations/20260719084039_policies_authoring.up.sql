-- T12.1: policy AUTHORING surface over the T2.4 evaluation substrate.
-- Adds versioning, the structured rule, the warn-first promote-to-enforce date,
-- a description, and the audit-event link. A separate append-only history table
-- retains every version so a one-click audited revert is possible.

-- The authored structured rule (G9: structured so the platform can evaluate it,
-- not prose). Distinct from `config` (kind runtime data); starts empty.
ALTER TABLE policies ADD COLUMN rule jsonb NOT NULL DEFAULT '{}';
ALTER TABLE policies ADD COLUMN description text;
ALTER TABLE policies ADD COLUMN version integer NOT NULL DEFAULT 1;
ALTER TABLE policies ADD COLUMN enforce_from timestamptz;      -- warn-first promote-to-enforce date
ALTER TABLE policies ADD COLUMN last_change_event text;         -- evt_… audit ref (G7)
ALTER TABLE policies ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- Rule policies use warn|enforce (warn-first default posture); the ai-assistant
-- kind keeps enabled|opt_in|disabled. Widen the T2.4 CHECK to the full contract
-- Enforcement enum.
ALTER TABLE policies DROP CONSTRAINT policies_enforcement_check;
ALTER TABLE policies ADD CONSTRAINT policies_enforcement_check
    CHECK (enforcement IN ('enabled', 'opt_in', 'disabled', 'warn', 'enforce'));

-- Append-only version history: one row per version ever written, so
-- "versions retained" and audited revert are real, not reconstructed.
CREATE TABLE policy_versions (
    id            text PRIMARY KEY,             -- pv_<hex>
    policy_id     text NOT NULL REFERENCES policies (id) ON DELETE CASCADE,
    version       integer NOT NULL,
    key           text NOT NULL,
    enforcement   text NOT NULL,
    rule          jsonb NOT NULL DEFAULT '{}',
    config        jsonb NOT NULL DEFAULT '{}',
    description   text,
    enforce_from  timestamptz,
    change_event  text,                          -- the evt_… that produced this version
    changed_by    text NOT NULL,                 -- actor (user or token id)
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (policy_id, version)
);

CREATE INDEX policy_versions_policy_idx ON policy_versions (policy_id, version DESC);
