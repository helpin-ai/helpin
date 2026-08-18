package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateAgentKnowledgeSourceSchema applies idempotent schema changes that
// AutoMigrate cannot express for scoped docs knowledge sources.
func MigrateAgentKnowledgeSourceSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.agent_knowledge_sources') IS NOT NULL THEN
        ALTER TABLE agent_knowledge_sources
            ADD COLUMN IF NOT EXISTS scope_type text NOT NULL DEFAULT 'space',
            ADD COLUMN IF NOT EXISTS collection_id uuid,
            ADD COLUMN IF NOT EXISTS document_id uuid;

        UPDATE agent_knowledge_sources
           SET scope_type = 'space'
         WHERE scope_type IS NULL OR scope_type = '';

        DROP INDEX IF EXISTS idx_aks_agent_space;

        CREATE UNIQUE INDEX IF NOT EXISTS idx_aks_agent_scope
            ON agent_knowledge_sources (
                agent_id,
                scope_type,
                space_id,
                COALESCE(collection_id::text, ''),
                COALESCE(document_id::text, '')
            );

        CREATE INDEX IF NOT EXISTS idx_aks_collection_id
            ON agent_knowledge_sources (collection_id)
            WHERE collection_id IS NOT NULL;

        CREATE INDEX IF NOT EXISTS idx_aks_document_id
            ON agent_knowledge_sources (document_id)
            WHERE document_id IS NOT NULL;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate agent knowledge source schema: %w", err)
	}
	return nil
}
