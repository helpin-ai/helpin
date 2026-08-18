ALTER TABLE dock_chats
    ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'private',
    ADD COLUMN IF NOT EXISTS module_id TEXT;

-- Existing chats opened from Support already belong to shared work. Make
-- their history visible to teammates who can access Support; unrelated chats
-- remain private.
UPDATE dock_chats
SET visibility = 'module',
    module_id = 'support'
WHERE support_conversation_id IS NOT NULL;

-- Before module_id existed, the first run still retained the page context in
-- input.additional_context. Use that immutable context to give recognizable
-- historical CRM, Projects, and Docs chats the same behavior as new chats.
-- Anything that cannot be classified reliably stays private.
WITH first_chat_context AS (
    SELECT DISTINCT ON (run.dock_chat_id)
        run.dock_chat_id,
        run.input->>'additional_context' AS additional_context
    FROM agent_runs AS run
    WHERE run.dock_chat_id IS NOT NULL
      AND NULLIF(run.input->>'additional_context', '') IS NOT NULL
    ORDER BY run.dock_chat_id, run.created_at ASC, run.id ASC
), classified_chat_context AS (
    SELECT
        dock_chat_id,
        CASE
            WHEN additional_context LIKE '%"entity_type":"support_conversation"%' THEN 'support'
            WHEN additional_context LIKE '%"entity_type":"crm_contact"%'
              OR additional_context LIKE '%"entity_type":"crm_deal"%' THEN 'crm'
            WHEN additional_context LIKE '%"entity_type":"task"%'
              OR additional_context LIKE '%"entity_type":"epic"%'
              OR additional_context LIKE '%"entity_type":"repository"%' THEN 'pm'
            WHEN additional_context LIKE '%"entity_type":"document"%' THEN 'docs'
            ELSE NULL
        END AS module_id
    FROM first_chat_context
)
UPDATE dock_chats AS chat
SET visibility = 'module',
    module_id = classified.module_id
FROM classified_chat_context AS classified
WHERE chat.id = classified.dock_chat_id
  AND chat.support_conversation_id IS NULL
  AND classified.module_id IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'dock_chats_visibility_valid' AND conrelid = 'dock_chats'::regclass
    ) THEN
        ALTER TABLE dock_chats
            ADD CONSTRAINT dock_chats_visibility_valid
            CHECK (visibility IN ('private', 'module', 'workspace'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'dock_chats_module_visibility_valid' AND conrelid = 'dock_chats'::regclass
    ) THEN
        ALTER TABLE dock_chats
            ADD CONSTRAINT dock_chats_module_visibility_valid
            CHECK (
                module_id IS NULL
                OR module_id IN ('support', 'crm', 'pm', 'docs')
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'dock_chats_module_required' AND conrelid = 'dock_chats'::regclass
    ) THEN
        ALTER TABLE dock_chats
            ADD CONSTRAINT dock_chats_module_required
            CHECK (visibility <> 'module' OR module_id IS NOT NULL);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_dock_chats_visibility
    ON dock_chats (workspace_id, visibility);

CREATE INDEX IF NOT EXISTS idx_dock_chats_module_visibility
    ON dock_chats (workspace_id, module_id, visibility)
    WHERE archived_at IS NULL;
