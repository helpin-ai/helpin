ALTER TABLE support_channel_accounts DROP CONSTRAINT IF EXISTS support_channel_accounts_provider_check;
ALTER TABLE support_channel_accounts ADD CONSTRAINT support_channel_accounts_provider_check CHECK (provider IN ('telegram','slack'));
CREATE UNIQUE INDEX IF NOT EXISTS slack_live_workspace ON support_channel_accounts(workspace_id) WHERE provider='slack' AND disabled_at IS NULL;
CREATE TABLE IF NOT EXISTS slack_install_states (
  state_hash text PRIMARY KEY, workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS slack_channel_routes (
  account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
  channel_id text NOT NULL, channel_name text NOT NULL, purpose text NOT NULL CHECK (purpose IN ('support','mixed','pm')),
  mailbox_id uuid REFERENCES support_mailboxes(id) ON DELETE CASCADE,
  team_id uuid REFERENCES workspace_teams(id) ON DELETE CASCADE, epic_id uuid REFERENCES pm_epics(id) ON DELETE SET NULL,
  PRIMARY KEY(account_id,channel_id)
);
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS provider_thread_key text;
ALTER TABLE support_conversations ADD COLUMN IF NOT EXISTS provider_channel_name text;
CREATE UNIQUE INDEX IF NOT EXISTS support_channel_thread ON support_conversations(channel_account_id, provider_thread_key) WHERE provider_thread_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS slack_members (
  account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
  slack_user text NOT NULL, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY(account_id,slack_user), UNIQUE(account_id,user_id)
);
CREATE TABLE IF NOT EXISTS slack_link_grants (
  token_hash text PRIMARY KEY, account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
  slack_user text NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS slack_task_threads (
  account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
  channel_id text NOT NULL, thread_ts text NOT NULL, task_id uuid NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
  comment_id uuid REFERENCES pm_comments(id) ON DELETE SET NULL,
  PRIMARY KEY(account_id,channel_id,thread_ts)
);
CREATE TABLE IF NOT EXISTS slack_task_drafts (
  id uuid PRIMARY KEY, account_id uuid NOT NULL REFERENCES support_channel_accounts(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  conversation_id uuid REFERENCES support_conversations(id) ON DELETE CASCADE,
  content jsonb NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS slack_update_requests (
  conversation_id uuid PRIMARY KEY REFERENCES support_conversations(id) ON DELETE CASCADE,
  published_key text NOT NULL
);

ALTER TABLE slack_task_threads ADD COLUMN IF NOT EXISTS card_ts text;
ALTER TABLE pm_comments ADD COLUMN IF NOT EXISTS slack_source_key text;
ALTER TABLE pm_comments ADD COLUMN IF NOT EXISTS slack_discussion_channel text;
CREATE UNIQUE INDEX IF NOT EXISTS slack_comment_source ON pm_comments(slack_source_key) WHERE slack_source_key IS NOT NULL;

-- Queue identifiers only. The worker rechecks the route, audience, and current
-- record before rendering anything for Slack.
CREATE OR REPLACE FUNCTION slack_enqueue_task_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO support_inbound_jobs(id,kind,workspace_id,channel_account_id,payload,status,available_at)
  SELECT gen_random_uuid(),'slack_pm_delivery',t.workspace_id,l.account_id,
         json_build_object('task',l.task_id,'channel',l.channel_id,'root',l.thread_ts,'completed',
           CASE WHEN TG_OP='UPDATE' THEN NEW.completed AND NOT OLD.completed ELSE false END)::text,'pending',now()
    FROM slack_task_threads l JOIN pm_tasks t ON t.id=l.task_id
    JOIN slack_channel_routes r ON r.account_id=l.account_id AND r.channel_id=l.channel_id AND r.purpose='pm'
    WHERE l.task_id=NEW.id;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_task_update ON pm_tasks;
CREATE TRIGGER slack_task_update AFTER UPDATE OF name,workflow_state_id,completed,blocked,deadline,team_id ON pm_tasks
FOR EACH ROW WHEN (OLD.name IS DISTINCT FROM NEW.name OR OLD.workflow_state_id IS DISTINCT FROM NEW.workflow_state_id OR OLD.completed IS DISTINCT FROM NEW.completed OR OLD.blocked IS DISTINCT FROM NEW.blocked OR OLD.deadline IS DISTINCT FROM NEW.deadline OR OLD.team_id IS DISTINCT FROM NEW.team_id) EXECUTE FUNCTION slack_enqueue_task_update();

CREATE OR REPLACE FUNCTION slack_enqueue_linked_comment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO support_inbound_jobs(id,kind,workspace_id,channel_account_id,payload,status,available_at)
  SELECT gen_random_uuid(),'slack_pm_delivery',t.workspace_id,l.account_id,
         json_build_object('task',l.task_id,'channel',l.channel_id,'root',l.thread_ts,'comment',NEW.id)::text,'pending',now()
    FROM slack_task_threads l JOIN pm_tasks t ON t.id=l.task_id
    JOIN slack_channel_routes r ON r.account_id=l.account_id AND r.channel_id=l.channel_id AND r.purpose='pm'
    WHERE l.comment_id=NEW.parent_id AND l.task_id=NEW.entity_id;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_discussion_reply ON pm_comments;
CREATE TRIGGER slack_discussion_reply AFTER INSERT ON pm_comments
FOR EACH ROW WHEN (NEW.parent_id IS NOT NULL AND NEW.entity_type='task' AND NEW.slack_source_key IS NULL) EXECUTE FUNCTION slack_enqueue_linked_comment();

-- Freeze the selected thread while the message and its merge audience are saved.
ALTER TABLE support_channel_outbox ADD COLUMN IF NOT EXISTS provider_origin_id uuid REFERENCES support_conversations(id) ON DELETE SET NULL;
CREATE OR REPLACE FUNCTION slack_pin_reply_thread() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    requested text;
    origins uuid[];
    has_slack boolean;
    target support_conversations%ROWTYPE;
    live boolean;
BEGIN
    SELECT metadata->>'provider_origin_id' INTO requested FROM support_messages WHERE id=NEW.message_id;
    WITH RECURSIVE members AS (
        SELECT id, channel, channel_identity_id FROM support_conversations
        WHERE id=NEW.conversation_id AND workspace_id=NEW.workspace_id
        UNION
        SELECT c.id, c.channel, c.channel_identity_id FROM support_conversations c JOIN members m ON c.merged_into_id=m.id
        WHERE c.workspace_id=NEW.workspace_id
    )
    SELECT array_agg(id) FILTER (WHERE channel_identity_id IS NOT NULL), bool_or(channel='slack')
      INTO origins,has_slack FROM members;
    IF NOT coalesce(has_slack,false) THEN RETURN NEW; END IF;
    IF requested IS NULL AND cardinality(origins)=1 THEN requested := origins[1]::text; END IF;
    SELECT * INTO target FROM support_conversations
      WHERE id=ANY(origins) AND id::text=requested AND workspace_id=NEW.workspace_id AND anonymized_at IS NULL;
    IF target.id IS NULL THEN
        NEW.dispatch_status := 'cancelled';
        NEW.last_error_code := 'select_destination';
        RETURN NEW;
    END IF;
    NEW.provider_origin_id := target.id;
    NEW.channel_account_id := target.channel_account_id;
    NEW.channel_identity_id := target.channel_identity_id;
    SELECT disabled_at IS NULL AND status='active' INTO live FROM support_channel_accounts WHERE id=target.channel_account_id;
    NEW.dispatch_status := CASE WHEN live THEN 'pending' ELSE 'cancelled' END;
    NEW.last_error_code := CASE WHEN live THEN NULL ELSE 'disconnected' END;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_pin_reply ON support_channel_outbox;
CREATE TRIGGER slack_pin_reply BEFORE INSERT ON support_channel_outbox FOR EACH ROW EXECUTE FUNCTION slack_pin_reply_thread();

CREATE OR REPLACE FUNCTION slack_clear_private_drafts() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM slack_task_drafts WHERE conversation_id=NEW.id;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_anonymized_conversation ON support_conversations;
CREATE TRIGGER slack_anonymized_conversation AFTER UPDATE OF anonymized_at ON support_conversations
FOR EACH ROW WHEN (OLD.anonymized_at IS NULL AND NEW.anonymized_at IS NOT NULL) EXECUTE FUNCTION slack_clear_private_drafts();

CREATE OR REPLACE FUNCTION slack_queue_support_handoff() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO support_inbound_jobs(id,kind,workspace_id,channel_account_id,conversation_id,payload,status,available_at)
    SELECT gen_random_uuid(),'slack_pm_delivery',c.workspace_id,c.channel_account_id,c.id,
           json_build_object('task',NEW.id,'support',c.id)::text,'pending',now()
    FROM support_conversations c JOIN support_channel_accounts a ON a.id=c.channel_account_id
    WHERE c.linked_task_id=NEW.id AND c.workspace_id=NEW.workspace_id AND c.channel='slack'
      AND c.assigned_user_id IS NOT NULL AND c.anonymized_at IS NULL AND a.disabled_at IS NULL;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_task_handoff ON pm_tasks;
CREATE TRIGGER slack_task_handoff AFTER UPDATE OF completed ON pm_tasks
FOR EACH ROW WHEN (NEW.completed AND NOT OLD.completed) EXECUTE FUNCTION slack_queue_support_handoff();
