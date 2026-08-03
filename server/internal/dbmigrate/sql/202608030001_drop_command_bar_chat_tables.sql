-- Dock cutover: the command-bar chat layer (threads/messages/proposals and
-- unmet-intent review) was replaced by dock chats backed by agent-runtime
-- chat-mode runs (dock_chats + agent_run_messages). Old chat history is not
-- migrated by design.

DROP TABLE IF EXISTS command_bar_messages;
DROP TABLE IF EXISTS command_bar_threads;
DROP TABLE IF EXISTS command_bar_unmet_intents;
