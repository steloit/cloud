-- T13.3: the AI assistant conversation store. Threads + messages are RETAINED
-- when the ai-assistant policy is disabled — the surface is HIDDEN (404
-- empty-equivalent, AI Law 4), never deleted; re-enable restores verbatim.
-- Ownership is user+org: a thread belongs to the user who opened it, scoped to
-- an org (the disable gate keys on that org's policy).
CREATE TABLE assistant_threads (
    id               text PRIMARY KEY,          -- thr_<hex>
    org_id           text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    user_id          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title            text NOT NULL DEFAULT '',
    context          jsonb NOT NULL DEFAULT '{}'::jsonb,  -- the scope chip: org/project/env/page/entity
    attached_insight text,                       -- ins_… (AI9), nullable
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_threads_owner ON assistant_threads (user_id, org_id, created_at DESC);

CREATE TABLE assistant_messages (
    id          text PRIMARY KEY,               -- msg_<hex>
    thread_id   text NOT NULL REFERENCES assistant_threads (id) ON DELETE CASCADE,
    role        text NOT NULL CHECK (role IN ('user', 'assistant')),
    content     text NOT NULL,
    evidence    jsonb NOT NULL DEFAULT '[]'::jsonb, -- Law 2: citable refs
    proposal_id text,                            -- when the turn produced one
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_messages_thread ON assistant_messages (thread_id, created_at);
