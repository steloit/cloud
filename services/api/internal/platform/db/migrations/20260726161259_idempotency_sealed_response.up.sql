-- US-3.6a (founder-ratified): a recorded response may contain CREDENTIAL
-- material — signup's session cookie, createWebhook's reveal-once signing
-- secret — so it is stored envelope-encrypted, never in plaintext.
--
-- The sealed payload is the whole response (headers + body), so a replay is
-- byte-for-byte equivalent from the client's perspective: a replayed signup
-- carries the SAME Set-Cookie, without which the client believes it is signed
-- in while holding no session.
--
-- Same crypto path as every other secret at rest (secrets.Seal/Open, AES-256-GCM
-- under a per-value DEK wrapped by the KEK). kek_id marks which key wrapped the
-- row, matching the vault's lazy-rotation model.
ALTER TABLE idempotency_keys
    DROP COLUMN response,
    ADD COLUMN response_ciphertext  bytea,
    ADD COLUMN response_nonce       bytea,
    ADD COLUMN response_wrapped_dek bytea,
    ADD COLUMN response_dek_nonce   bytea,
    ADD COLUMN response_kek_id      text;
