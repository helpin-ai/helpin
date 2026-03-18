package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateAgentRunTargets backfills agent_runs.target_type and target_id from
// the legacy story_id / ticket_id columns so that the NOT NULL constraint
// added by AutoMigrate succeeds.
func MigrateAgentRunTargets(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.agent_runs') IS NOT NULL THEN
        -- Add target_type/target_id as nullable first if they don't exist
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agent_runs' AND column_name='target_id') THEN
            ALTER TABLE agent_runs ADD COLUMN target_id uuid;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agent_runs' AND column_name='target_type') THEN
            ALTER TABLE agent_runs ADD COLUMN target_type text;
        END IF;

        -- Backfill from story_id
        IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agent_runs' AND column_name='story_id') THEN
            UPDATE agent_runs
               SET target_type = 'story', target_id = story_id::uuid
             WHERE target_id IS NULL AND story_id IS NOT NULL;
        END IF;

        -- Backfill from ticket_id (now called conversation_id)
        IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='agent_runs' AND column_name='ticket_id') THEN
            UPDATE agent_runs
               SET target_type = 'support_conversation', target_id = ticket_id::uuid
             WHERE target_id IS NULL AND ticket_id IS NOT NULL;
        END IF;

        -- Rename legacy target_type values
        UPDATE agent_runs
           SET target_type = 'support_conversation'
         WHERE target_type = 'support_ticket';

        -- Default any remaining nulls so NOT NULL constraint can be applied
        UPDATE agent_runs
           SET target_type = 'story'
         WHERE target_type IS NULL;

        DELETE FROM agent_runs WHERE target_id IS NULL;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate agent run targets: %w", err)
	}
	return nil
}
