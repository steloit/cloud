ALTER TABLE idempotency_keys
    DROP COLUMN response_ciphertext,
    DROP COLUMN response_nonce,
    DROP COLUMN response_wrapped_dek,
    DROP COLUMN response_dek_nonce,
    DROP COLUMN response_kek_id,
    ADD COLUMN response bytea;
