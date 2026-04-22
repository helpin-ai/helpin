ALTER TABLE user_passkeys
    DROP CONSTRAINT IF EXISTS user_passkeys_credential_id_key;

DROP INDEX IF EXISTS user_passkeys_credential_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS uni_user_passkeys_credential_id
    ON user_passkeys (credential_id);
