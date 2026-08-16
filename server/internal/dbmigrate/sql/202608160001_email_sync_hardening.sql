-- Make Gmail ingestion idempotent at the database boundary and repair any
-- duplicates created before these constraints existed.

CREATE TEMP TABLE email_thread_dedup ON COMMIT DROP AS
SELECT id AS duplicate_id,
       FIRST_VALUE(id) OVER (
           PARTITION BY email_account_id, thread_external_id
           ORDER BY created_at, id
       ) AS keeper_id
FROM crm_email_threads
WHERE thread_external_id <> '';

UPDATE crm_email_messages AS message
SET thread_id = dedup.keeper_id
FROM email_thread_dedup AS dedup
WHERE message.thread_id = dedup.duplicate_id
  AND dedup.duplicate_id <> dedup.keeper_id;

DELETE FROM crm_email_threads AS thread
USING email_thread_dedup AS dedup
WHERE thread.id = dedup.duplicate_id
  AND dedup.duplicate_id <> dedup.keeper_id;

CREATE TEMP TABLE email_message_dedup ON COMMIT DROP AS
SELECT id AS duplicate_id,
       FIRST_VALUE(id) OVER (
           PARTITION BY email_account_id, message_external_id
           ORDER BY created_at, id
       ) AS keeper_id
FROM crm_email_messages
WHERE message_external_id IS NOT NULL
  AND message_external_id <> '';

INSERT INTO crm_email_message_contacts (
    message_id,
    contact_id,
    participant_role,
    workspace_id,
    created_at
)
SELECT dedup.keeper_id,
       association.contact_id,
       association.participant_role,
       association.workspace_id,
       association.created_at
FROM crm_email_message_contacts AS association
JOIN email_message_dedup AS dedup
  ON dedup.duplicate_id = association.message_id
WHERE dedup.duplicate_id <> dedup.keeper_id
ON CONFLICT (message_id, contact_id, participant_role) DO NOTHING;

DELETE FROM crm_email_messages AS message
USING email_message_dedup AS dedup
WHERE message.id = dedup.duplicate_id
  AND dedup.duplicate_id <> dedup.keeper_id;

UPDATE crm_email_threads AS thread
SET message_count = summary.message_count,
    last_message_at = summary.last_message_at,
    updated_at = NOW()
FROM (
    SELECT thread_id,
           COUNT(*)::integer AS message_count,
           MAX(sent_at) AS last_message_at
    FROM crm_email_messages
    WHERE thread_id IS NOT NULL
    GROUP BY thread_id
) AS summary
WHERE thread.id = summary.thread_id;

UPDATE crm_email_threads AS thread
SET message_count = 0,
    contact_ids = '[]'::jsonb,
    updated_at = NOW()
WHERE NOT EXISTS (
    SELECT 1
    FROM crm_email_messages AS message
    WHERE message.thread_id = thread.id
);

UPDATE crm_email_threads AS thread
SET contact_ids = COALESCE(contacts.contact_ids, '[]'::jsonb),
    updated_at = NOW()
FROM (
    SELECT message.thread_id,
           jsonb_agg(DISTINCT association.contact_id::text) AS contact_ids
    FROM crm_email_messages AS message
    JOIN crm_email_message_contacts AS association
      ON association.message_id = message.id
    WHERE message.thread_id IS NOT NULL
    GROUP BY message.thread_id
) AS contacts
WHERE thread.id = contacts.thread_id;

DELETE FROM crm_email_accounts AS account
USING (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY oauth_state ORDER BY created_at DESC, id DESC) AS row_number
    FROM crm_email_accounts
    WHERE oauth_state IS NOT NULL
) AS duplicate
WHERE account.id = duplicate.id
  AND duplicate.row_number > 1
  AND account.status = 'pending_oauth';

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_threads_account_external_unique
    ON crm_email_threads(email_account_id, thread_external_id)
    WHERE thread_external_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_messages_account_external_unique
    ON crm_email_messages(email_account_id, message_external_id)
    WHERE message_external_id IS NOT NULL AND message_external_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_accounts_oauth_state_unique
    ON crm_email_accounts(oauth_state)
    WHERE oauth_state IS NOT NULL;
