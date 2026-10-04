ALTER TABLE dock_chats ADD COLUMN IF NOT EXISTS coverage_gap_id uuid;
ALTER TABLE dock_chats ADD COLUMN IF NOT EXISTS initial_context jsonb;

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_coverage_gaps_ws_id
    ON support_coverage_gaps(workspace_id, id);
ALTER TABLE dock_chats ADD CONSTRAINT dock_chats_coverage_gap_fk
    FOREIGN KEY (workspace_id, coverage_gap_id) REFERENCES support_coverage_gaps(workspace_id, id) ON DELETE CASCADE;
ALTER TABLE dock_chats ADD CONSTRAINT dock_chats_coverage_scope
    CHECK (coverage_gap_id IS NULL OR (
        support_conversation_id IS NULL AND visibility = 'module' AND module_id = 'support'
    ));
CREATE UNIQUE INDEX IF NOT EXISTS idx_dock_chats_active_coverage_gap
    ON dock_chats(workspace_id, coverage_gap_id)
    WHERE coverage_gap_id IS NOT NULL AND archived_at IS NULL;
