CREATE TABLE IF NOT EXISTS crm_mailbox_sending_policies (
 mailbox_key text PRIMARY KEY, daily_limit bigint NOT NULL DEFAULT 100,
 manual_reserve bigint NOT NULL DEFAULT 10, min_interval_seconds bigint NOT NULL DEFAULT 60,
 cooldown_until timestamptz,
 CHECK (daily_limit BETWEEN 1 AND 1000), CHECK (manual_reserve >= 0 AND manual_reserve < daily_limit),
 CHECK (min_interval_seconds BETWEEN 60 AND 3600)
);
CREATE TABLE IF NOT EXISTS crm_email_send_reservations (
 id text PRIMARY KEY, mailbox_key text NOT NULL, workspace_id text, account_id text,
 sequence_id text, automated boolean, first_email boolean, status text NOT NULL, created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS crm_send_reservation_mailbox_window ON crm_email_send_reservations (mailbox_key,created_at);
CREATE INDEX IF NOT EXISTS crm_send_reservation_sequence_window ON crm_email_send_reservations (sequence_id,created_at);
ALTER TABLE crm_email_sequences ADD COLUMN IF NOT EXISTS daily_new_recipients bigint NOT NULL DEFAULT 25;
CREATE INDEX IF NOT EXISTS crm_enrollment_due_mailbox ON crm_sequence_enrollments (account_id,next_at) WHERE status IN ('active','waiting_task','sending');

-- Preserve the last day's existing attempts so rollout cannot reset sending capacity.
INSERT INTO crm_email_send_reservations
 (id,mailbox_key,workspace_id,account_id,sequence_id,automated,first_email,status,created_at)
SELECT d.id::text, a.provider || ':' || lower(trim(a.email_address)), d.workspace_id::text,
 d.account_id::text, e.sequence_id::text, true,
 NOT EXISTS (SELECT 1 FROM jsonb_array_elements(e.steps) WITH ORDINALITY AS s(step,position)
             WHERE s.position <= d.step_index AND s.step->>'kind' = 'email'),
 CASE WHEN d.status = 'sent' THEN 'sent' ELSE 'pending' END, d.created_at
FROM crm_sequence_deliveries d
JOIN crm_email_accounts a ON a.id = d.account_id
JOIN crm_sequence_enrollments e ON e.id = d.enrollment_id
WHERE d.kind = 'email' AND d.created_at > now() - interval '24 hours'
 AND d.status NOT IN ('deferred','rejected')
ON CONFLICT (id) DO NOTHING;

-- Seed other recorded outbound mail conservatively; mirrored mailbox connections count once.
INSERT INTO crm_email_send_reservations
 (id,mailbox_key,workspace_id,account_id,sequence_id,automated,first_email,status,created_at)
SELECT DISTINCT ON (a.provider,lower(trim(a.email_address)),COALESCE(NULLIF(m.message_external_id,''),m.id::text))
 'legacy-' || md5(a.provider || ':' || lower(trim(a.email_address)) || ':' || COALESCE(NULLIF(m.message_external_id,''),m.id::text)),
 a.provider || ':' || lower(trim(a.email_address)), m.workspace_id::text, a.id::text, '', false, false, 'sent', m.sent_at
FROM crm_email_messages m JOIN crm_email_accounts a ON a.id = m.email_account_id
WHERE m.direction = 'outbound' AND m.sent_at > now() - interval '24 hours'
 AND NOT EXISTS (
   SELECT 1 FROM crm_sequence_deliveries d
   LEFT JOIN crm_email_messages delivered ON delivered.id::text = d.result_id
   WHERE d.result_id = m.id::text OR (delivered.message_external_id = m.message_external_id AND delivered.from_address = m.from_address)
 )
ON CONFLICT (id) DO NOTHING;
