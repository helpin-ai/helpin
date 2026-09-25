-- Records the outcome of explicit operator checks for instance-level
-- capabilities (for example a test email through the application mail sender).
-- config_fingerprint identifies the configuration that was tested, so a result
-- no longer applies once that configuration changes. No secrets are stored.
CREATE TABLE IF NOT EXISTS instance_capability_checks (
    key text PRIMARY KEY,
    ok boolean NOT NULL,
    error text,
    config_fingerprint text NOT NULL,
    checked_by uuid,
    checked_at timestamptz NOT NULL DEFAULT now()
);
