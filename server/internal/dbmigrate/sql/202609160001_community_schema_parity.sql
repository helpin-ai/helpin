-- Keep fresh ledger installs consistent with the API model schema. Existing
-- installation policies are unchanged; EE explicitly chooses enforced on create.
ALTER TABLE support_widget_installations ALTER COLUMN identity_verification_mode SET DEFAULT 'report_only';
-- Canonicalize the existing constraint after historical varchar migrations.
ALTER TABLE support_mailboxes DROP CONSTRAINT IF EXISTS support_mailboxes_reply_preset_chk;
ALTER TABLE support_mailboxes ADD CONSTRAINT support_mailboxes_reply_preset_chk
CHECK (reply_time_preset IS NULL OR reply_time_preset::text = ANY (ARRAY['few_minutes'::varchar::text, 'few_hours'::varchar::text, 'same_day'::varchar::text, 'custom'::varchar::text]));
