-- dbmigrate:no-transaction
-- Indexes the conversation merge and dock chat models declare. The cascade
-- from support_coverage_gaps to dock_chats also uses the coverage gap index.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_dock_chats_coverage_gap_id ON dock_chats (coverage_gap_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_merges_workspace_id ON support_conversation_merges (workspace_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_merges_operation_id ON support_conversation_merges (operation_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_merges_source_id ON support_conversation_merges (source_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_merges_target_id ON support_conversation_merges (target_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_merges_unmerge_operation_id ON support_conversation_merges (unmerge_operation_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_merge_jobs_workspace_id ON support_merge_jobs (workspace_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_merge_jobs_available_at ON support_merge_jobs (available_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_merge_messages_merge_id ON support_merge_messages (merge_id);
