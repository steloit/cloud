DROP TABLE recovery_codes;
DROP TABLE mfa_totp;
ALTER TABLE users DROP COLUMN mfa_enabled;
