-- Instance-wide GitHub App created through the GitHub App manifest flow.
-- At most one row: singleton is always true and unique. Private key, client
-- secret and webhook secret are AES-256-GCM ciphertext (GIT_OAUTH_ENCRYPTION_KEY).
-- GITHUB_APP_* environment variables, when set, take precedence over this row.
CREATE TABLE IF NOT EXISTS github_app_credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    singleton boolean NOT NULL DEFAULT true CHECK (singleton),
    app_id text NOT NULL,
    slug text NOT NULL,
    name text NOT NULL DEFAULT '',
    client_id text,
    client_secret_encrypted text,
    private_key_encrypted text NOT NULL,
    webhook_secret_encrypted text,
    html_url text,
    owner_login text,
    owner_type text,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_github_app_credentials_singleton
    ON github_app_credentials (singleton);
