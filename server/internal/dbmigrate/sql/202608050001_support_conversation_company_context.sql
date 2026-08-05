-- Migration: support_conversation_company_context
-- Store mutable company context on widget sessions and the stable company
-- captured by support conversations. Existing sessions remain unassigned.

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS crm_company_id uuid;

ALTER TABLE support_widget_sessions
    ADD COLUMN IF NOT EXISTS crm_company_id uuid;

CREATE INDEX IF NOT EXISTS idx_support_conversations_crm_company_id
    ON support_conversations (crm_company_id);

CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_crm_company_id
    ON support_widget_sessions (crm_company_id);

-- Historical conversations can be linked without ambiguity only when their
-- contact has exactly one contact-to-company association in the workspace.
WITH contact_company AS (
    SELECT workspace_id, from_object_id AS contact_id, to_object_id AS company_id
    FROM crm_associations
    WHERE from_object_type = 'contact'
      AND to_object_type = 'company'
    UNION ALL
    SELECT workspace_id, to_object_id AS contact_id, from_object_id AS company_id
    FROM crm_associations
    WHERE from_object_type = 'company'
      AND to_object_type = 'contact'
), association AS (
    SELECT workspace_id, contact_id, MIN(company_id::text)::uuid AS company_id
    FROM contact_company
    GROUP BY workspace_id, contact_id
    HAVING COUNT(DISTINCT company_id) = 1
)
UPDATE support_conversations AS conversation
SET crm_company_id = association.company_id
FROM association
JOIN crm_companies AS company
  ON company.id = association.company_id
 AND company.workspace_id = association.workspace_id
WHERE conversation.crm_company_id IS NULL
  AND conversation.crm_contact_id = association.contact_id
  AND conversation.workspace_id = association.workspace_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_support_conversations_crm_company'
          AND conrelid = 'support_conversations'::regclass
    ) THEN
        ALTER TABLE support_conversations
            ADD CONSTRAINT fk_support_conversations_crm_company
            FOREIGN KEY (crm_company_id) REFERENCES crm_companies(id)
            ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_support_widget_sessions_crm_company'
          AND conrelid = 'support_widget_sessions'::regclass
    ) THEN
        ALTER TABLE support_widget_sessions
            ADD CONSTRAINT fk_support_widget_sessions_crm_company
            FOREIGN KEY (crm_company_id) REFERENCES crm_companies(id)
            ON DELETE SET NULL;
    END IF;
END $$;
