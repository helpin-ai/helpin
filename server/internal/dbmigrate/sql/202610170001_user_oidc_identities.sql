-- OpenID Connect sign-in: a user is identified by the provider (issuer) plus its
-- stable subject claim, never by email.
CREATE TABLE IF NOT EXISTS user_oidc_identities (
 id uuid DEFAULT gen_random_uuid(),
 user_id uuid NOT NULL,
 issuer text NOT NULL,
 subject text NOT NULL,
 created_at timestamptz,
 PRIMARY KEY (id),
 CONSTRAINT fk_user_oidc_identities_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_oidc_identities_issuer_subject ON user_oidc_identities (issuer, subject);
