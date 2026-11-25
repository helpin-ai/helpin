-- Refresh existing Slack conversation cards when the priority changes.
DROP TRIGGER IF EXISTS slack_feed_conversation ON support_conversations;
CREATE TRIGGER slack_feed_conversation AFTER INSERT OR UPDATE OF status,assigned_user_id,assigned_agent_id,flow_state,mailbox_id,snoozed_until,subject,merged_into_id,anonymized_at,priority
 ON support_conversations FOR EACH ROW EXECUTE FUNCTION helpin_enqueue_slack_feed();
