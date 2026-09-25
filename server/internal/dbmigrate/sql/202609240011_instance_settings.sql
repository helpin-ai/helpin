-- Server administration for self-hosted (Community) installations.
--
-- instance_settings holds at most one row (singleton is always true and
-- unique): the signup policy, whether the first account has already claimed
-- server admin, and the application SMTP settings saved from the UI. The SMTP
-- password is AES-256-GCM ciphertext (CRM_ENCRYPTION_KEY). SMTP_* and
-- POSTMARK_* environment variables, when set, take precedence over the stored
-- SMTP settings.
--
-- users.is_server_admin marks the accounts that administer the server itself
-- (signup policy, server admins, application email). It is separate from
-- users.is_platform_admin, which is the hosted operator role.
-- users.signup_verification_pending marks accounts created through
-- domain-restricted signup that must verify their email before signing in.
--
-- Enterprise builds create the same schema but never read or set these fields.

CREATE TABLE IF NOT EXISTS instance_settings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    singleton boolean NOT NULL DEFAULT true CHECK (singleton),
    signup_mode text NOT NULL DEFAULT 'invite_only'
        CHECK (signup_mode IN ('open', 'invite_only', 'domains')),
    signup_allowed_domains text NOT NULL DEFAULT '',
    admin_bootstrapped_at timestamptz,
    smtp_host text,
    smtp_port integer,
    smtp_username text,
    smtp_password_encrypted text,
    smtp_from text,
    smtp_tls_mode text,
    smtp_updated_at timestamptz,
    updated_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_instance_settings_singleton
    ON instance_settings (singleton);

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_server_admin boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS signup_verification_pending boolean NOT NULL DEFAULT false;

-- A fresh server (no accounts yet) starts invite-only: its first account
-- becomes the server admin and everyone after that needs an invitation.
-- Existing servers keep open signup so nobody is locked out by the upgrade;
-- their first-account claim is already spent, and the API assigns their
-- server admin at startup (see InstanceService.Bootstrap).
INSERT INTO instance_settings (singleton, signup_mode, admin_bootstrapped_at)
SELECT true,
       CASE WHEN EXISTS (SELECT 1 FROM users) THEN 'open' ELSE 'invite_only' END,
       CASE WHEN EXISTS (SELECT 1 FROM users) THEN now() ELSE NULL END
ON CONFLICT (singleton) DO NOTHING;
