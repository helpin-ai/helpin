-- Task -> Story rollback for the 202604010002 hard-cut rename.
--
-- WARNING:
-- This script is intentionally NOT embedded in the runtime migration runner.
-- Use it only for a controlled rollback together with story-era application
-- code and a release that runs the API with RUN_AUTO_MIGRATE=false.

DO $$
BEGIN
    IF to_regclass('public.pm_tasks') IS NOT NULL AND to_regclass('public.pm_stories') IS NULL THEN
        ALTER TABLE pm_tasks RENAME TO pm_stories;
    END IF;
    IF to_regclass('public.pm_task_owners') IS NOT NULL AND to_regclass('public.pm_story_owners') IS NULL THEN
        ALTER TABLE pm_task_owners RENAME TO pm_story_owners;
    END IF;
    IF to_regclass('public.pm_task_followers') IS NOT NULL AND to_regclass('public.pm_story_followers') IS NULL THEN
        ALTER TABLE pm_task_followers RENAME TO pm_story_followers;
    END IF;
    IF to_regclass('public.pm_task_labels') IS NOT NULL AND to_regclass('public.pm_story_labels') IS NULL THEN
        ALTER TABLE pm_task_labels RENAME TO pm_story_labels;
    END IF;
    IF to_regclass('public.pm_task_links') IS NOT NULL AND to_regclass('public.pm_story_links') IS NULL THEN
        ALTER TABLE pm_task_links RENAME TO pm_story_links;
    END IF;
    IF to_regclass('public.pm_task_templates') IS NOT NULL AND to_regclass('public.pm_story_templates') IS NULL THEN
        ALTER TABLE pm_task_templates RENAME TO pm_story_templates;
    END IF;
    IF to_regclass('public.task_delivery_targets') IS NOT NULL AND to_regclass('public.story_delivery_targets') IS NULL THEN
        ALTER TABLE task_delivery_targets RENAME TO story_delivery_targets;
    END IF;
    IF to_regclass('public.task_git_links') IS NOT NULL AND to_regclass('public.story_git_links') IS NULL THEN
        ALTER TABLE task_git_links RENAME TO story_git_links;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.pm_task_display_id_seq') IS NOT NULL AND to_regclass('public.pm_story_display_id_seq') IS NULL THEN
        ALTER SEQUENCE pm_task_display_id_seq RENAME TO pm_story_display_id_seq;
    END IF;

    IF to_regclass('public.pm_story_display_id_seq') IS NOT NULL
       AND EXISTS (
           SELECT 1
           FROM information_schema.columns
           WHERE table_schema = 'public' AND table_name = 'pm_stories' AND column_name = 'display_id'
       ) THEN
        ALTER SEQUENCE pm_story_display_id_seq OWNED BY pm_stories.display_id;
        ALTER TABLE pm_stories ALTER COLUMN display_id SET DEFAULT nextval('pm_story_display_id_seq');
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_stories' AND column_name = 'task_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_stories' AND column_name = 'story_type'
    ) THEN
        ALTER TABLE pm_stories RENAME COLUMN task_type TO story_type;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_teams' AND column_name = 'default_task_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'workspace_teams' AND column_name = 'default_story_type'
    ) THEN
        ALTER TABLE workspace_teams RENAME COLUMN default_task_type TO default_story_type;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_team_field_visibility' AND column_name = 'task_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_team_field_visibility' AND column_name = 'story_type'
    ) THEN
        ALTER TABLE pm_team_field_visibility RENAME COLUMN task_type TO story_type;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_owners' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_owners' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE pm_story_owners RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_followers' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_followers' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE pm_story_followers RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_labels' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_labels' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE pm_story_labels RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_checklist_items' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_checklist_items' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE pm_checklist_items RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_external_links' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_external_links' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE pm_external_links RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_links' AND column_name = 'source_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_links' AND column_name = 'source_story_id'
    ) THEN
        ALTER TABLE pm_story_links RENAME COLUMN source_task_id TO source_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_links' AND column_name = 'target_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_links' AND column_name = 'target_story_id'
    ) THEN
        ALTER TABLE pm_story_links RENAME COLUMN target_task_id TO target_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_templates' AND column_name = 'task_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_story_templates' AND column_name = 'story_type'
    ) THEN
        ALTER TABLE pm_story_templates RENAME COLUMN task_type TO story_type;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_templates' AND column_name = 'created_from_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_templates' AND column_name = 'created_from_story_id'
    ) THEN
        ALTER TABLE pm_recurring_templates RENAME COLUMN created_from_task_id TO created_from_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_templates' AND column_name = 'last_generated_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_templates' AND column_name = 'last_generated_story_id'
    ) THEN
        ALTER TABLE pm_recurring_templates RENAME COLUMN last_generated_task_id TO last_generated_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_runs' AND column_name = 'generated_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_recurring_runs' AND column_name = 'generated_story_id'
    ) THEN
        ALTER TABLE pm_recurring_runs RENAME COLUMN generated_task_id TO generated_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'support_conversations' AND column_name = 'linked_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'support_conversations' AND column_name = 'linked_story_id'
    ) THEN
        ALTER TABLE support_conversations RENAME COLUMN linked_task_id TO linked_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'active_task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'active_story_id'
    ) THEN
        ALTER TABLE agents RENAME COLUMN active_task_id TO active_story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_runs' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_runs' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE agent_runs RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_handoffs' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_handoffs' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE agent_handoffs RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'story_delivery_targets' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'story_delivery_targets' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE story_delivery_targets RENAME COLUMN task_id TO story_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'story_git_links' AND column_name = 'task_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'story_git_links' AND column_name = 'story_id'
    ) THEN
        ALTER TABLE story_git_links RENAME COLUMN task_id TO story_id;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.idx_pm_tasks_workspace') IS NOT NULL AND to_regclass('public.idx_pm_stories_workspace') IS NULL THEN
        ALTER INDEX idx_pm_tasks_workspace RENAME TO idx_pm_stories_workspace;
    END IF;
    IF to_regclass('public.idx_pm_tasks_state') IS NOT NULL AND to_regclass('public.idx_pm_stories_state') IS NULL THEN
        ALTER INDEX idx_pm_tasks_state RENAME TO idx_pm_stories_state;
    END IF;
    IF to_regclass('public.idx_pm_tasks_epic') IS NOT NULL AND to_regclass('public.idx_pm_stories_epic') IS NULL THEN
        ALTER INDEX idx_pm_tasks_epic RENAME TO idx_pm_stories_epic;
    END IF;
    IF to_regclass('public.idx_pm_tasks_sprint') IS NOT NULL AND to_regclass('public.idx_pm_stories_sprint') IS NULL THEN
        ALTER INDEX idx_pm_tasks_sprint RENAME TO idx_pm_stories_sprint;
    END IF;
    IF to_regclass('public.idx_pm_tasks_team') IS NOT NULL AND to_regclass('public.idx_pm_stories_team') IS NULL THEN
        ALTER INDEX idx_pm_tasks_team RENAME TO idx_pm_stories_team;
    END IF;
    IF to_regclass('public.idx_pm_tasks_owner') IS NOT NULL AND to_regclass('public.idx_pm_stories_owner') IS NULL THEN
        ALTER INDEX idx_pm_tasks_owner RENAME TO idx_pm_stories_owner;
    END IF;
    IF to_regclass('public.idx_pm_tasks_display_id') IS NOT NULL AND to_regclass('public.idx_pm_stories_display_id') IS NULL THEN
        ALTER INDEX idx_pm_tasks_display_id RENAME TO idx_pm_stories_display_id;
    END IF;
    IF to_regclass('public.idx_pm_tasks_workflow_state_position') IS NOT NULL AND to_regclass('public.idx_pm_stories_workflow_state_position') IS NULL THEN
        ALTER INDEX idx_pm_tasks_workflow_state_position RENAME TO idx_pm_stories_workflow_state_position;
    END IF;
    IF to_regclass('public.idx_pm_task_owners_user') IS NOT NULL AND to_regclass('public.idx_pm_story_owners_user') IS NULL THEN
        ALTER INDEX idx_pm_task_owners_user RENAME TO idx_pm_story_owners_user;
    END IF;
    IF to_regclass('public.idx_pm_task_followers_user') IS NOT NULL AND to_regclass('public.idx_pm_story_followers_user') IS NULL THEN
        ALTER INDEX idx_pm_task_followers_user RENAME TO idx_pm_story_followers_user;
    END IF;
    IF to_regclass('public.idx_pm_task_labels_label') IS NOT NULL AND to_regclass('public.idx_pm_story_labels_label') IS NULL THEN
        ALTER INDEX idx_pm_task_labels_label RENAME TO idx_pm_story_labels_label;
    END IF;
    IF to_regclass('public.idx_pm_tasks_plan_document_id') IS NOT NULL AND to_regclass('public.idx_pm_stories_plan_document_id') IS NULL THEN
        ALTER INDEX idx_pm_tasks_plan_document_id RENAME TO idx_pm_stories_plan_document_id;
    END IF;
    IF to_regclass('public.idx_pm_tasks_owner_member_id') IS NOT NULL AND to_regclass('public.idx_pm_stories_owner_member_id') IS NULL THEN
        ALTER INDEX idx_pm_tasks_owner_member_id RENAME TO idx_pm_stories_owner_member_id;
    END IF;
    IF to_regclass('public.idx_pm_tasks_requester_member_id') IS NOT NULL AND to_regclass('public.idx_pm_stories_requester_member_id') IS NULL THEN
        ALTER INDEX idx_pm_tasks_requester_member_id RENAME TO idx_pm_stories_requester_member_id;
    END IF;
END $$;

DO $$
DECLARE
    constraint_name text;
BEGIN
    IF to_regclass('public.pm_comments') IS NOT NULL THEN
        SELECT c.conname
          INTO constraint_name
          FROM pg_constraint c
          JOIN pg_class t ON t.oid = c.conrelid
          JOIN pg_namespace n ON n.oid = t.relnamespace
         WHERE n.nspname = 'public'
           AND t.relname = 'pm_comments'
           AND c.contype = 'c'
           AND pg_get_constraintdef(c.oid) ILIKE '%entity_type%'
         LIMIT 1;

        IF constraint_name IS NOT NULL THEN
            EXECUTE format('ALTER TABLE pm_comments DROP CONSTRAINT %I', constraint_name);
        END IF;

        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint c
              JOIN pg_class t ON t.oid = c.conrelid
              JOIN pg_namespace n ON n.oid = t.relnamespace
             WHERE n.nspname = 'public'
               AND t.relname = 'pm_comments'
               AND c.conname = 'pm_comments_entity_type_check'
        ) THEN
            ALTER TABLE pm_comments
                ADD CONSTRAINT pm_comments_entity_type_check
                CHECK (entity_type IN ('story', 'epic', 'doc'));
        END IF;
    END IF;

    IF to_regclass('public.agent_runs') IS NOT NULL
       AND EXISTS (
           SELECT 1
           FROM information_schema.columns
           WHERE table_schema = 'public' AND table_name = 'agent_runs' AND column_name = 'target_type'
       ) THEN
        ALTER TABLE agent_runs ALTER COLUMN target_type SET DEFAULT 'story';
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.pm_comments') IS NOT NULL THEN
        UPDATE pm_comments SET entity_type = 'story' WHERE entity_type = 'task';
    END IF;

    IF to_regclass('public.pm_activity_log') IS NOT NULL THEN
        UPDATE pm_activity_log SET entity_type = 'story' WHERE entity_type = 'task';
        IF EXISTS (
            SELECT 1
              FROM information_schema.columns
             WHERE table_schema = 'public'
               AND table_name = 'pm_activity_log'
               AND column_name = 'event_type'
        ) THEN
            UPDATE pm_activity_log
               SET event_type = regexp_replace(event_type, '^task\.', 'story.')
             WHERE event_type LIKE 'task.%';
        END IF;
    END IF;

    IF to_regclass('public.pm_attachments') IS NOT NULL THEN
        UPDATE pm_attachments SET entity_type = 'story' WHERE entity_type = 'task';
    END IF;

    IF to_regclass('public.notifications') IS NOT NULL THEN
        UPDATE notifications SET entity_type = 'story' WHERE entity_type = 'task';
        UPDATE notifications
           SET event_type = regexp_replace(event_type, '^task\.', 'story.')
         WHERE event_type LIKE 'task.%';
    END IF;

    IF to_regclass('public.notification_events') IS NOT NULL THEN
        UPDATE notification_events
           SET event_type = regexp_replace(event_type, '^task\.', 'story.')
         WHERE event_type LIKE 'task.%';
    END IF;

    IF to_regclass('public.entity_followers') IS NOT NULL THEN
        UPDATE entity_followers SET entity_type = 'story' WHERE entity_type = 'task';
    END IF;

    IF to_regclass('public.crm_associations') IS NOT NULL THEN
        UPDATE crm_associations SET from_object_type = 'story' WHERE from_object_type = 'task';
        UPDATE crm_associations SET to_object_type = 'story' WHERE to_object_type = 'task';
    END IF;

    IF to_regclass('public.docs_links') IS NOT NULL THEN
        UPDATE docs_links SET linked_object_type = 'story' WHERE linked_object_type = 'task';
    END IF;

    IF to_regclass('public.agents') IS NOT NULL THEN
        UPDATE agents
           SET allowed_targets = COALESCE((
                SELECT jsonb_agg(to_jsonb(CASE WHEN elem = 'task' THEN 'story' ELSE elem END) ORDER BY ord)
                  FROM jsonb_array_elements_text(COALESCE(allowed_targets, '[]'::jsonb)) WITH ORDINALITY AS arr(elem, ord)
           ), '[]'::jsonb)
         WHERE COALESCE(allowed_targets, '[]'::jsonb) @> '["task"]'::jsonb;

        UPDATE agents SET preset_key = 'story_planner' WHERE preset_key = 'task_planner';
        UPDATE agents SET source_preset_key = 'story_planner' WHERE source_preset_key = 'task_planner';
        UPDATE agents
           SET preset_version_key = replace(preset_version_key, 'task_planner', 'story_planner')
         WHERE preset_version_key LIKE '%task_planner%';
        UPDATE agents
           SET source_preset_version_key = replace(source_preset_version_key, 'task_planner', 'story_planner')
         WHERE source_preset_version_key LIKE '%task_planner%';
        UPDATE agents SET name = 'Story Planner' WHERE name = 'Task Planner';
    END IF;

    IF to_regclass('public.workspace_agent_preset_versions') IS NOT NULL THEN
        UPDATE workspace_agent_preset_versions
           SET family_key = 'story_planner'
         WHERE family_key = 'task_planner';
        UPDATE workspace_agent_preset_versions
           SET version_key = replace(version_key, 'task_planner', 'story_planner')
         WHERE version_key LIKE '%task_planner%';
        UPDATE workspace_agent_preset_versions
           SET source_version_key = replace(source_version_key, 'task_planner', 'story_planner')
         WHERE source_version_key LIKE '%task_planner%';
        UPDATE workspace_agent_preset_versions
           SET label = replace(label, 'Task Planner', 'Story Planner')
         WHERE label LIKE '%Task Planner%';
    END IF;

    IF to_regclass('public.agent_runs') IS NOT NULL THEN
        UPDATE agent_runs SET target_type = 'story' WHERE target_type = 'task';
    END IF;

    IF to_regclass('public.pm_recurring_templates') IS NOT NULL THEN
        UPDATE pm_recurring_templates
           SET seed_payload = replace(seed_payload::text, '"task_type":', '"story_type":')::jsonb
         WHERE seed_payload::text LIKE '%"task_type":%';
    END IF;
END $$;
