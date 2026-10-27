-- People answer customer portal requests. AI may only draft a private
-- suggestion for the teammate, so the customer-facing 'ai_first' portal mode
-- is retired. Workspaces that used it keep AI help as private suggestions.
UPDATE support_widget_installations
SET settings = jsonb_set(settings, '{portal_ai_mode}', '"internal_note"')
WHERE settings->>'portal_ai_mode' = 'ai_first';

-- Queued work follows the same rule.
UPDATE support_portal_ai_dispatches
SET mode_at_enqueue = 'internal_note', updated_at = now()
WHERE mode_at_enqueue = 'ai_first' AND status IN ('pending', 'publishing');

-- Open portal requests still waiting on AI go to the team, as a teammate's
-- "Pause AI" would leave them, so no customer waits on an AI that will no
-- longer reply.
UPDATE support_conversations
SET flow_state = CASE WHEN assigned_user_id IS NOT NULL OR opened_by_user_id IS NOT NULL
        THEN 'assigned_to_human' ELSE 'waiting_for_human' END,
    human_takeover = true,
    assigned_agent_id = NULL,
    ai_paused_at = COALESCE(ai_paused_at, now()),
    needs_human_reply = true,
    updated_at = now()
WHERE channel = 'portal' AND flow_state = 'ai_handling' AND status NOT IN ('resolved', 'closed', 'spam');

ALTER TABLE support_portal_ai_dispatches DROP CONSTRAINT IF EXISTS chk_support_portal_ai_dispatch_mode;
ALTER TABLE support_portal_ai_dispatches ADD CONSTRAINT chk_support_portal_ai_dispatch_mode
    CHECK (mode_at_enqueue = 'internal_note' OR status IN ('published', 'failed')) NOT VALID;
ALTER TABLE support_portal_ai_dispatches VALIDATE CONSTRAINT chk_support_portal_ai_dispatch_mode;
