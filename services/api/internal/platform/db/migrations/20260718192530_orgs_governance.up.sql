-- T2.7: org governance — full orgs shape (models.md), subscriptions (row
-- exists, inert — billing attaches in E11), invites, ≥1-owner trigger.

-- orgs grows to the models.md shape; slug is immutable (enforced in code:
-- updateOrg never writes it) and unique. Existing rows (test worlds only)
-- backfill slug from id.
ALTER TABLE orgs
    ADD COLUMN slug text,
    ADD COLUMN home_region text NOT NULL DEFAULT 'aws/ap-south-1',
    ADD COLUMN plan text NOT NULL DEFAULT 'free'
        CHECK (plan IN ('free', 'pro', 'business', 'enterprise')),
    ADD COLUMN deletion_scheduled_at timestamptz;
UPDATE orgs SET slug = id WHERE slug IS NULL;
ALTER TABLE orgs
    ALTER COLUMN slug SET NOT NULL,
    ADD CONSTRAINT orgs_slug_key UNIQUE (slug);

-- subscriptions: the billing lifecycle row (dunning enum from models.md).
-- Inert until E11; created with the org so billing has a row to attach to.
CREATE TABLE subscriptions (
    org_id        text PRIMARY KEY REFERENCES orgs (id) ON DELETE CASCADE,
    plan          text NOT NULL DEFAULT 'free'
        CHECK (plan IN ('free', 'pro', 'business', 'enterprise')),
    anchor_day    int NOT NULL DEFAULT 1 CHECK (anchor_day BETWEEN 1 AND 28),
    status        text NOT NULL DEFAULT 'current'
        CHECK (status IN ('current', 'grace', 'provisioning_paused', 'suspended')),
    trial_ends_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- invites: the id IS the public link (inv_<hex>, unguessable). Expiry is
-- computed (expires_at < now → expired), never a cron flipping rows.
CREATE TABLE invites (
    id         text PRIMARY KEY,               -- inv_<hex>
    org_id     text NOT NULL REFERENCES orgs (id) ON DELETE CASCADE,
    email      text NOT NULL,
    role       text NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'billing')),
    status     text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'revoked')),
    inviter_id text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX invites_pending_dedupe
    ON invites (org_id, lower(email)) WHERE status = 'pending';
CREATE INDEX invites_expires_idx ON invites (expires_at);

-- ≥1 owner enforced by trigger (models.md): the last owner can neither be
-- demoted nor removed. During an org CASCADE delete the parent row is already
-- gone — the guard steps aside so deletion proceeds.
CREATE FUNCTION members_keep_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.role <> 'owner' THEN
        RETURN COALESCE(NEW, OLD);
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.role = 'owner' THEN
        RETURN NEW;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = OLD.org_id) THEN
        RETURN COALESCE(NEW, OLD); -- org cascade in flight
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM members
        WHERE org_id = OLD.org_id AND role = 'owner' AND id <> OLD.id
    ) THEN
        RAISE EXCEPTION 'last owner: an organization keeps at least one owner (F1)'
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER members_last_owner_guard
    BEFORE UPDATE OF role OR DELETE ON members
    FOR EACH ROW EXECUTE FUNCTION members_keep_owner();
