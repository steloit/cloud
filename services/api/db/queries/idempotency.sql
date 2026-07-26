-- US-3.6 / S7: idempotent mutating POSTs.

-- name: ClaimIdempotencyKey :one
-- ATOMIC claim: insert the key or return nothing if it already exists. Doing
-- this as one statement (not read-then-write) is what makes a concurrent
-- double-submit resolve to exactly one winner — the loser gets no row back and
-- replays the winner's response.
-- Two expiries, not one, because a claim and a completed response mean
-- different things:
--   * COMPLETED (status_code NOT NULL) — holds for 24h; that is the replay
--     window the S7 ruling promises.
--   * IN FLIGHT (status_code NULL) — holds only briefly. LOAD-BEARING: this
--     interval is what bounds double-execution. A predecessor still running
--     when a successor takes over means two handler invocations exist at once;
--     the claim_token fence keeps the RECORD correct but cannot un-create the
--     second resource. Safe only while it exceeds the maximum handler lifetime
--     (ReadTimeout 30s + WriteTimeout 30s + the 5s completion budget), so this
--     value must not be lowered below that without revisiting the guarantee.
--
--     Holding a dead claim for the full 24h would instead strand the client on
--     a key it can never use again (US-3.6: a failure must never strand state),
--     so an abandoned claim becomes re-claimable after 5 minutes.
INSERT INTO idempotency_keys (principal, endpoint, key, body_sha256, claim_token)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (principal, endpoint, key) DO UPDATE
    SET body_sha256 = EXCLUDED.body_sha256,
        claim_token = EXCLUDED.claim_token,
        status_code = NULL,
        response    = NULL,
        created_at  = now()
    WHERE (idempotency_keys.status_code IS NOT NULL
             AND idempotency_keys.created_at < now() - interval '24 hours')
       OR (idempotency_keys.status_code IS NULL
             AND idempotency_keys.created_at < now() - interval '5 minutes')
RETURNING *;

-- name: GetIdempotencyKey :one
-- The existing (live) entry for a replay or a body-mismatch check.
SELECT * FROM idempotency_keys
WHERE principal = $1 AND endpoint = $2 AND key = $3
  AND created_at >= now() - interval '24 hours';

-- name: SaveIdempotentResponse :execrows
-- Record the original response so a replay returns it verbatim. FENCED on
-- claim_token: a request that outran the abandonment window and returns late
-- must not overwrite the record of the successor that took the claim over.
UPDATE idempotency_keys SET status_code = $5, response = $6
WHERE principal = $1 AND endpoint = $2 AND key = $3 AND claim_token = $4;

-- name: ReleaseIdempotencyKey :execrows
-- Drop a claim whose request FAILED. Without this the key stays held until it
-- ages out and an immediate retry is refused as an in-flight duplicate — the
-- client would be stranded by its own failed attempt. Only ever called for a
-- non-2xx, so a recorded response is never destroyed.
-- Fenced on claim_token for the same reason SaveIdempotentResponse is: a late
-- loser releasing the winner's live claim would let a third request execute
-- concurrently with it.
DELETE FROM idempotency_keys
WHERE principal = $1 AND endpoint = $2 AND key = $3
  AND claim_token = $4 AND status_code IS NULL;

-- name: SweepExpiredIdempotencyKeys :execrows
-- The 24h window is a PROMISE, not just a read filter: without a sweep the
-- table grows without bound and every recorded response body is retained
-- forever, long past the window the contract states.
DELETE FROM idempotency_keys WHERE created_at < now() - interval '24 hours';
