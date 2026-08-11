-- Restore verification for historical routes with real inbound-email evidence.
-- Provider confirmation messages prove address ownership, not that forwarding is enabled.
WITH evidence AS (
    SELECT
        email_log.email_route_id,
        MAX(email_log.created_at) AS verified_at
    FROM support_email_logs AS email_log
    WHERE email_log.direction = 'inbound'
      AND email_log.email_route_id IS NOT NULL
      AND NOT (
          LOWER(TRIM(COALESCE(email_log.from_email, ''))) = 'forwarding-noreply@google.com'
          OR LOWER(COALESCE(email_log.raw_body, '')) LIKE '%mail-settings.google.com/mail/vf-%'
          OR (
              LOWER(COALESCE(email_log.from_email, '')) LIKE '%zoho%'
              AND LOWER(COALESCE(email_log.subject, '')) LIKE '%forward%'
              AND (
                  LOWER(COALESCE(email_log.subject, '')) LIKE '%confirm%'
                  OR LOWER(COALESCE(email_log.subject, '')) LIKE '%verif%'
              )
          )
      )
    GROUP BY email_log.email_route_id
)
UPDATE support_email_routes AS route
SET forwarding_verified_at = evidence.verified_at
FROM evidence
WHERE route.id = evidence.email_route_id
  AND route.forwarding_verified_at IS NULL;
