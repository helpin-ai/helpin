ALTER TABLE slack_channel_routes DROP CONSTRAINT IF EXISTS slack_channel_routes_purpose_check;
ALTER TABLE slack_channel_routes ADD CONSTRAINT slack_channel_routes_purpose_check CHECK (purpose IN ('support','mixed','pm','inbox'));

-- 'shared' identifies Helpin's main inbox, whose conversations have a null mailbox_id.
CREATE TABLE IF NOT EXISTS slack_feed_inboxes (
 account_id uuid NOT NULL, channel_id text NOT NULL, inbox_key text NOT NULL,
 PRIMARY KEY(account_id,inbox_key),
 FOREIGN KEY(account_id,channel_id) REFERENCES slack_channel_routes(account_id,channel_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS slack_feed_destination ON slack_feed_inboxes(account_id,channel_id);
CREATE TABLE IF NOT EXISTS slack_feed_threads (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), account_id uuid NOT NULL, channel_id text NOT NULL,
 conversation_id uuid NOT NULL REFERENCES support_conversations(id) ON DELETE CASCADE,
 root_ts text, permalink text, active boolean NOT NULL DEFAULT true,
 UNIQUE(account_id,channel_id,conversation_id), UNIQUE(account_id,channel_id,root_ts),
 FOREIGN KEY(account_id) REFERENCES support_channel_accounts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS slack_feed_conversation ON slack_feed_threads(conversation_id);
CREATE TABLE IF NOT EXISTS slack_feed_receipts (
 thread_id uuid NOT NULL REFERENCES slack_feed_threads(id) ON DELETE CASCADE,
 part_key text NOT NULL, slack_ts text NOT NULL,
 PRIMARY KEY(thread_id,part_key)
);
CREATE OR REPLACE FUNCTION helpin_enqueue_slack_feed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE c support_conversations%ROWTYPE; mid uuid; bind boolean := false;
BEGIN
 IF TG_TABLE_NAME='support_messages' THEN
   SELECT * INTO c FROM support_conversations WHERE id=NEW.conversation_id;
   mid := NEW.id;
 ELSIF TG_TABLE_NAME IN ('support_channel_outbox', 'support_attachments') THEN
   mid := NEW.message_id;
   IF mid IS NULL THEN RETURN NEW; END IF;
   SELECT c0.* INTO c FROM support_conversations c0 JOIN support_messages m ON m.conversation_id=c0.id WHERE m.id=mid;
 ELSE
   c := NEW;
   bind := TG_OP='INSERT' OR OLD.mailbox_id IS DISTINCT FROM NEW.mailbox_id;
 END IF;
 IF bind AND c.anonymized_at IS NULL AND c.merged_into_id IS NULL AND c.status<>'spam' THEN
   INSERT INTO slack_feed_threads(account_id,channel_id,conversation_id)
   SELECT f.account_id,f.channel_id,c.id FROM slack_feed_inboxes f
    JOIN support_channel_accounts a ON a.id=f.account_id AND a.workspace_id=c.workspace_id AND a.disabled_at IS NULL
    WHERE f.inbox_key=coalesce(c.mailbox_id::text,'shared')
   ON CONFLICT(account_id,channel_id,conversation_id) DO UPDATE SET active=true;
 END IF;
 INSERT INTO support_inbound_jobs(id,kind,workspace_id,conversation_id,message_id,channel_account_id,payload,status,available_at)
 SELECT gen_random_uuid(),'slack_inbox_feed',c.workspace_id,c.id,mid,t.account_id,json_build_object('thread',t.id)::text,'pending',now()
 FROM slack_feed_threads t WHERE t.conversation_id=c.id AND t.active;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS slack_feed_conversation ON support_conversations;
CREATE TRIGGER slack_feed_conversation AFTER INSERT OR UPDATE OF status,assigned_user_id,assigned_agent_id,flow_state,mailbox_id,snoozed_until,subject,merged_into_id,anonymized_at
 ON support_conversations FOR EACH ROW EXECUTE FUNCTION helpin_enqueue_slack_feed();
DROP TRIGGER IF EXISTS slack_feed_message ON support_messages;
CREATE TRIGGER slack_feed_message AFTER INSERT OR UPDATE OF metadata,deleted_at ON support_messages
 FOR EACH ROW EXECUTE FUNCTION helpin_enqueue_slack_feed();

DROP TRIGGER IF EXISTS slack_feed_delivery ON support_channel_outbox;
CREATE TRIGGER slack_feed_delivery AFTER UPDATE OF dispatch_status,receipt_status ON support_channel_outbox
 FOR EACH ROW WHEN (OLD.dispatch_status IS DISTINCT FROM NEW.dispatch_status OR OLD.receipt_status IS DISTINCT FROM NEW.receipt_status)
 EXECUTE FUNCTION helpin_enqueue_slack_feed();
DROP TRIGGER IF EXISTS slack_feed_attachment ON support_attachments;
CREATE TRIGGER slack_feed_attachment AFTER INSERT OR UPDATE OF message_id,is_uploaded,processing_status ON support_attachments
 FOR EACH ROW WHEN (NEW.message_id IS NOT NULL) EXECUTE FUNCTION helpin_enqueue_slack_feed();

CREATE OR REPLACE FUNCTION helpin_stop_slack_feed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE slack_feed_threads SET active=false WHERE account_id=OLD.account_id AND channel_id=OLD.channel_id;
 RETURN OLD;
END $$;
DROP TRIGGER IF EXISTS slack_feed_remove_route ON slack_channel_routes;
CREATE TRIGGER slack_feed_remove_route AFTER DELETE ON slack_channel_routes FOR EACH ROW EXECUTE FUNCTION helpin_stop_slack_feed();
