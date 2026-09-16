-- Informational absence emails stay in history without changing workload,
-- unread counts, inbox ordering or the last real customer/agent reply.
DROP TRIGGER IF EXISTS support_message_state_inserted ON support_messages;
CREATE TRIGGER support_message_state_inserted
AFTER INSERT ON support_messages
FOR EACH ROW
WHEN (NEW.message_type <> 'email_notice')
EXECUTE FUNCTION project_support_message_state();
