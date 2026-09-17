# Forwarding Verification Backfill Design

Existing email routes became orange when `forwarding_verified_at` was introduced because historical routes started with that field empty. A route's `last_inbound_at` alone is not sufficient evidence: a provider confirmation email can set it before forwarding is enabled.

Add one idempotent database migration that marks a historical route verified only when `support_email_logs` contains a qualifying inbound message for that route. Gmail and Zoho confirmation messages are excluded using the same sender, subject, and confirmation-link signals as the inbound service. The latest qualifying log timestamp becomes `forwarding_verified_at`; routes with no qualifying evidence remain incomplete.

No UI fallback, cutoff date, provider OAuth, or user action is added. New routes continue using the round-trip test.

