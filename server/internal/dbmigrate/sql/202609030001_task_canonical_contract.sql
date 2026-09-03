-- Complete the Helpin-owned Story -> Task hard cut for persisted contracts.
-- Historical schema migrations and Shortcut's upstream wire vocabulary remain
-- unchanged; active application data must use Task identifiers exclusively.

CREATE OR REPLACE FUNCTION pg_temp.canonicalize_task_contract(payload jsonb, parent_key text DEFAULT NULL)
RETURNS jsonb
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    result jsonb;
    item_key text;
    canonical_key text;
    item_value jsonb;
    scalar_value text;
BEGIN
    IF payload IS NULL THEN
        RETURN NULL;
    END IF;

    CASE jsonb_typeof(payload)
    WHEN 'object' THEN
        result := '{}'::jsonb;
        FOR item_key, item_value IN SELECT key, value FROM jsonb_each(payload)
        LOOP
            canonical_key := CASE item_key
                WHEN 'story_id' THEN 'task_id'
                WHEN 'story_type' THEN 'task_type'
                WHEN 'story_refs' THEN 'task_refs'
                WHEN 'proposed_stories' THEN 'proposed_tasks'
                WHEN 'created_story_ids' THEN 'created_task_ids'
                WHEN 'story_plan_doc' THEN 'task_plan_doc'
                WHEN 'story_plan' THEN 'task_plan'
                ELSE item_key
            END;
            IF parent_key = 'channel_preferences' AND canonical_key LIKE 'story.%' THEN
                canonical_key := 'task.' || substring(canonical_key FROM 7);
            END IF;
            -- Prefer a value that already uses the canonical key when both exist.
            IF item_key <> canonical_key AND payload ? canonical_key THEN
                CONTINUE;
            END IF;
            result := result || jsonb_build_object(
                canonical_key,
                pg_temp.canonicalize_task_contract(item_value, canonical_key)
            );
        END LOOP;
        RETURN result;

    WHEN 'array' THEN
        SELECT COALESCE(
            jsonb_agg(pg_temp.canonicalize_task_contract(element, parent_key) ORDER BY ordinal),
            '[]'::jsonb
        )
          INTO result
          FROM jsonb_array_elements(payload) WITH ORDINALITY AS items(element, ordinal);
        RETURN result;

    WHEN 'string' THEN
        scalar_value := payload #>> '{}';
        scalar_value := CASE scalar_value
            WHEN 'publish_story_plan_doc' THEN 'publish_task_plan_doc'
            WHEN 'publish_story_plan' THEN 'publish_task_plan'
            WHEN 'story_plan_doc' THEN 'task_plan_doc'
            WHEN 'story_plan' THEN 'task_plan'
            WHEN 'add_story_comment' THEN 'add_task_comment'
            WHEN 'list_story_checklist' THEN 'list_task_checklist'
            WHEN 'update_story_state' THEN 'update_task_state'
            WHEN 'pm.create_followup_stories' THEN 'pm.create_followup_tasks'
            WHEN 'pm.story_completion_followups' THEN 'pm.task_completion_followups'
            WHEN 'story_planner' THEN 'task_planner'
            ELSE scalar_value
        END;
        IF scalar_value = 'story' AND parent_key IN (
            'target_type', 'entity_type', 'linked_object_type', 'object_type',
            'from_object_type', 'to_object_type', 'type', 'allowed_targets',
            'supported_target_types', 'target_types'
        ) THEN
            scalar_value := 'task';
        ELSIF scalar_value LIKE 'story.%' AND parent_key IN ('event_type', 'trigger_type') THEN
            scalar_value := 'task.' || substring(scalar_value FROM 7);
        END IF;
        RETURN to_jsonb(scalar_value);

    ELSE
        RETURN payload;
    END CASE;
END;
$$;

DO $$
BEGIN
    IF to_regclass('public.agents') IS NOT NULL THEN
        UPDATE agents
           SET allowed_targets = pg_temp.canonicalize_task_contract(allowed_targets, 'allowed_targets'),
               allowed_tools = pg_temp.canonicalize_task_contract(allowed_tools, 'allowed_tools'),
               allowed_commands = pg_temp.canonicalize_task_contract(allowed_commands, 'allowed_commands'),
               preset_key = CASE WHEN preset_key = 'story_planner' THEN 'task_planner' ELSE preset_key END,
               source_preset_key = CASE WHEN source_preset_key = 'story_planner' THEN 'task_planner' ELSE source_preset_key END,
               preset_version_key = replace(preset_version_key, 'story_planner', 'task_planner'),
               source_preset_version_key = replace(source_preset_version_key, 'story_planner', 'task_planner')
         WHERE allowed_targets::text ~ 'story'
            OR allowed_tools::text ~ 'story'
            OR allowed_commands::text ~ 'story'
            OR preset_key = 'story_planner'
            OR source_preset_key = 'story_planner'
            OR preset_version_key LIKE '%story_planner%'
            OR source_preset_version_key LIKE '%story_planner%';

        UPDATE agents
           SET system_prompt = regexp_replace(
                 regexp_replace(system_prompt, '\mStories\M', 'Tasks', 'g'),
                 '\mStory\M', 'Task', 'g'
               )
         WHERE system_prompt IS NOT NULL
           AND (preset_key = 'task_planner' OR source_preset_key = 'task_planner');
    END IF;

    IF to_regclass('public.agent_versions') IS NOT NULL THEN
        UPDATE agent_versions
           SET allowed_targets = pg_temp.canonicalize_task_contract(allowed_targets, 'allowed_targets'),
               allowed_tools = pg_temp.canonicalize_task_contract(allowed_tools, 'allowed_tools')
         WHERE allowed_targets::text ~ 'story'
            OR allowed_tools::text ~ 'story';
    END IF;

    IF to_regclass('public.agent_templates') IS NOT NULL THEN
        UPDATE agent_templates
           SET allowed_targets = pg_temp.canonicalize_task_contract(allowed_targets, 'allowed_targets'),
               allowed_tools = pg_temp.canonicalize_task_contract(allowed_tools, 'allowed_tools'),
               allowed_commands = pg_temp.canonicalize_task_contract(allowed_commands, 'allowed_commands')
         WHERE allowed_targets::text ~ 'story'
            OR allowed_tools::text ~ 'story'
            OR allowed_commands::text ~ 'story';
    END IF;

    IF to_regclass('public.workspace_agent_preset_versions') IS NOT NULL THEN
        UPDATE workspace_agent_preset_versions
           SET family_key = CASE WHEN family_key = 'story_planner' THEN 'task_planner' ELSE family_key END,
               version_key = replace(version_key, 'story_planner', 'task_planner'),
               source_version_key = replace(source_version_key, 'story_planner', 'task_planner'),
               allowed_targets = pg_temp.canonicalize_task_contract(allowed_targets, 'allowed_targets'),
               allowed_tools = pg_temp.canonicalize_task_contract(allowed_tools, 'allowed_tools')
         WHERE family_key = 'story_planner'
            OR version_key LIKE '%story_planner%'
            OR source_version_key LIKE '%story_planner%'
            OR allowed_targets::text ~ 'story'
            OR allowed_tools::text ~ 'story';

        UPDATE workspace_agent_preset_versions
           SET system_prompt = regexp_replace(
                 regexp_replace(system_prompt, '\mStories\M', 'Tasks', 'g'),
                 '\mStory\M', 'Task', 'g'
               )
         WHERE system_prompt IS NOT NULL
           AND family_key = 'task_planner';
    END IF;

    IF to_regclass('public.agent_runs') IS NOT NULL THEN
        UPDATE agent_runs
           SET target_type = 'task'
         WHERE target_type = 'story';
        UPDATE agent_runs
           SET input = pg_temp.canonicalize_task_contract(input),
               output_summary = pg_temp.canonicalize_task_contract(output_summary)
         WHERE input::text ~ 'story'
            OR output_summary::text ~ 'story';
    END IF;

    IF to_regclass('public.agent_trigger_executions') IS NOT NULL THEN
        UPDATE agent_trigger_executions
           SET trigger_type = CASE
                   WHEN trigger_type LIKE 'story.%' THEN 'task.' || substring(trigger_type FROM 7)
                   ELSE trigger_type
               END,
               reference_type = CASE WHEN reference_type = 'story' THEN 'task' ELSE reference_type END,
               target_type = CASE WHEN target_type = 'story' THEN 'task' ELSE target_type END
         WHERE trigger_type LIKE 'story.%'
            OR reference_type = 'story'
            OR target_type = 'story';
    END IF;

    IF to_regclass('public.agent_run_messages') IS NOT NULL THEN
        UPDATE agent_run_messages
           SET content_blocks = CASE WHEN content_blocks IS NULL THEN NULL ELSE pg_temp.canonicalize_task_contract(content_blocks) END,
               turn_segments = CASE WHEN turn_segments IS NULL THEN NULL ELSE pg_temp.canonicalize_task_contract(turn_segments) END,
               tool_invocations = CASE WHEN tool_invocations IS NULL THEN NULL ELSE pg_temp.canonicalize_task_contract(tool_invocations) END
         WHERE COALESCE(content_blocks::text, '') ~ 'story'
            OR COALESCE(turn_segments::text, '') ~ 'story'
            OR COALESCE(tool_invocations::text, '') ~ 'story';
    END IF;

    IF to_regclass('public.agent_run_interactions') IS NOT NULL THEN
        UPDATE agent_run_interactions
           SET request_payload = pg_temp.canonicalize_task_contract(request_payload),
               response_payload = CASE WHEN response_payload IS NULL THEN NULL ELSE pg_temp.canonicalize_task_contract(response_payload) END,
               runtime_metadata = pg_temp.canonicalize_task_contract(runtime_metadata)
         WHERE request_payload::text ~ 'story'
            OR COALESCE(response_payload::text, '') ~ 'story'
            OR runtime_metadata::text ~ 'story';
    END IF;

    IF to_regclass('public.agent_run_artifacts') IS NOT NULL THEN
        UPDATE agent_run_artifacts
           SET metadata = pg_temp.canonicalize_task_contract(metadata)
         WHERE metadata::text ~ 'story';
    END IF;

    IF to_regclass('public.automation_rules') IS NOT NULL THEN
        UPDATE automation_rules
           SET trigger_type = regexp_replace(trigger_type, '^story\.', 'task.'),
               trigger_config = pg_temp.canonicalize_task_contract(trigger_config),
               action_config = pg_temp.canonicalize_task_contract(action_config)
         WHERE trigger_type LIKE 'story.%'
            OR trigger_config::text ~ 'story'
            OR action_config::text ~ 'story';
    END IF;

    IF to_regclass('public.pm_recurring_templates') IS NOT NULL THEN
        UPDATE pm_recurring_templates
           SET seed_payload = pg_temp.canonicalize_task_contract(seed_payload)
         WHERE seed_payload::text ~ 'story';
    END IF;

    IF to_regclass('public.notifications') IS NOT NULL THEN
        UPDATE notifications
           SET entity_type = CASE WHEN entity_type = 'story' THEN 'task' ELSE entity_type END,
               event_type = CASE
                   WHEN event_type LIKE 'story.%' THEN 'task.' || substring(event_type FROM 7)
                   ELSE event_type
               END,
               metadata = pg_temp.canonicalize_task_contract(metadata),
               entity_snapshot = pg_temp.canonicalize_task_contract(entity_snapshot),
               parent_entity_snapshot = pg_temp.canonicalize_task_contract(parent_entity_snapshot)
         WHERE entity_type = 'story'
            OR event_type LIKE 'story.%'
            OR COALESCE(metadata::text, '') ~ 'story'
            OR COALESCE(entity_snapshot::text, '') ~ 'story'
            OR COALESCE(parent_entity_snapshot::text, '') ~ 'story';
    END IF;

    IF to_regclass('public.notification_events') IS NOT NULL THEN
        UPDATE notification_events
           SET event_type = CASE
                   WHEN event_type LIKE 'story.%' THEN 'task.' || substring(event_type FROM 7)
                   ELSE event_type
               END,
               metadata = pg_temp.canonicalize_task_contract(metadata)
         WHERE event_type LIKE 'story.%'
            OR COALESCE(metadata::text, '') ~ 'story';
    END IF;

    IF to_regclass('public.notification_preferences') IS NOT NULL THEN
        UPDATE notification_preferences
           SET channel_preferences = pg_temp.canonicalize_task_contract(channel_preferences, 'channel_preferences')
         WHERE channel_preferences::text ~ 'story';
    END IF;

    IF to_regclass('public.pm_activity_log') IS NOT NULL THEN
        UPDATE pm_activity_log
           SET entity_type = CASE WHEN entity_type = 'story' THEN 'task' ELSE entity_type END,
               event_type = CASE
                   WHEN event_type LIKE 'story.%' THEN 'task.' || substring(event_type FROM 7)
                   ELSE event_type
               END,
               metadata = pg_temp.canonicalize_task_contract(metadata)
         WHERE entity_type = 'story'
            OR event_type LIKE 'story.%'
            OR COALESCE(metadata::text, '') ~ 'story';
    END IF;

    IF to_regclass('public.pm_comments') IS NOT NULL THEN
        UPDATE pm_comments SET entity_type = 'task' WHERE entity_type = 'story';
    END IF;
    IF to_regclass('public.pm_attachments') IS NOT NULL THEN
        UPDATE pm_attachments SET entity_type = 'task' WHERE entity_type = 'story';
    END IF;
    IF to_regclass('public.entity_followers') IS NOT NULL THEN
        UPDATE entity_followers SET entity_type = 'task' WHERE entity_type = 'story';
    END IF;
    IF to_regclass('public.crm_associations') IS NOT NULL THEN
        UPDATE crm_associations SET from_object_type = 'task' WHERE from_object_type = 'story';
        UPDATE crm_associations SET to_object_type = 'task' WHERE to_object_type = 'story';
    END IF;
    IF to_regclass('public.docs_links') IS NOT NULL THEN
        UPDATE docs_links SET linked_object_type = 'task' WHERE linked_object_type = 'story';
    END IF;
    IF to_regclass('public.pm_external_links') IS NOT NULL THEN
        UPDATE pm_external_links SET entity_type = 'task' WHERE entity_type = 'story';
    END IF;
    IF to_regclass('public.crm_entity_summaries') IS NOT NULL THEN
        UPDATE crm_entity_summaries
           SET entity_type = 'task',
               metadata = pg_temp.canonicalize_task_contract(metadata)
         WHERE entity_type = 'story'
            OR COALESCE(metadata::text, '') ~ 'story';
    END IF;
END $$;

-- Reconcile columns recreated by AutoMigrate after the original hard cut. The
-- canonical column wins when it is populated; a legacy-only value is preserved,
-- and conflicting non-null IDs stop the migration instead of losing data.
CREATE OR REPLACE FUNCTION pg_temp.reconcile_task_id_column(
    target_table text,
    legacy_column text,
    canonical_column text
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    target_relation regclass;
    legacy_exists boolean;
    canonical_exists boolean;
    has_conflict boolean;
BEGIN
    target_relation := to_regclass(format('public.%I', target_table));
    IF target_relation IS NULL THEN
        RETURN;
    END IF;

    SELECT EXISTS (
        SELECT 1
          FROM pg_attribute
         WHERE attrelid = target_relation
           AND attname = legacy_column
           AND NOT attisdropped
    ) INTO legacy_exists;
    IF NOT legacy_exists THEN
        RETURN;
    END IF;

    SELECT EXISTS (
        SELECT 1
          FROM pg_attribute
         WHERE attrelid = target_relation
           AND attname = canonical_column
           AND NOT attisdropped
    ) INTO canonical_exists;
    IF NOT canonical_exists THEN
        EXECUTE format(
            'ALTER TABLE %s RENAME COLUMN %I TO %I',
            target_relation,
            legacy_column,
            canonical_column
        );
        RETURN;
    END IF;

    EXECUTE format(
        'SELECT EXISTS (SELECT 1 FROM %s WHERE %I IS NOT NULL AND %I IS NOT NULL AND %I IS DISTINCT FROM %I)',
        target_relation,
        legacy_column,
        canonical_column,
        legacy_column,
        canonical_column
    ) INTO has_conflict;
    IF has_conflict THEN
        RAISE EXCEPTION 'task canonicalization failed: %.% conflicts with %',
            target_table,
            legacy_column,
            canonical_column;
    END IF;

    EXECUTE format(
        'UPDATE %s SET %I = %I WHERE %I IS NULL AND %I IS NOT NULL',
        target_relation,
        canonical_column,
        legacy_column,
        canonical_column,
        legacy_column
    );
    EXECUTE format('ALTER TABLE %s DROP COLUMN %I', target_relation, legacy_column);
END;
$$;

SELECT pg_temp.reconcile_task_id_column('agents', 'active_story_id', 'active_task_id');
SELECT pg_temp.reconcile_task_id_column('agent_runs', 'story_id', 'task_id');
SELECT pg_temp.reconcile_task_id_column('agent_handoffs', 'story_id', 'task_id');
SELECT pg_temp.reconcile_task_id_column(
    'pm_recurring_templates',
    'created_from_story_id',
    'created_from_task_id'
);
SELECT pg_temp.reconcile_task_id_column(
    'pm_recurring_templates',
    'last_generated_story_id',
    'last_generated_task_id'
);
SELECT pg_temp.reconcile_task_id_column(
    'pm_recurring_runs',
    'generated_story_id',
    'generated_task_id'
);
SELECT pg_temp.reconcile_task_id_column(
    'support_conversations',
    'linked_story_id',
    'linked_task_id'
);

-- task_type is the application-owned field. A recreated story_type column may
-- contain its own default values, so do not overwrite live task_type values.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = 'public'
           AND table_name = 'pm_team_field_visibility'
           AND column_name = 'story_type'
    ) THEN
        IF EXISTS (
            SELECT 1
              FROM information_schema.columns
             WHERE table_schema = 'public'
               AND table_name = 'pm_team_field_visibility'
               AND column_name = 'task_type'
        ) THEN
            ALTER TABLE pm_team_field_visibility DROP COLUMN story_type;
        ELSE
            ALTER TABLE pm_team_field_visibility RENAME COLUMN story_type TO task_type;
        END IF;
    END IF;
END $$;

-- Recreated legacy tables must be empty. Refuse to discard rows if an older
-- application wrote to them after the original reconciliation migration.
DO $$
DECLARE
    legacy_table text;
    legacy_relation regclass;
    has_rows boolean;
BEGIN
    FOREACH legacy_table IN ARRAY ARRAY[
        'pm_stories',
        'pm_story_followers',
        'pm_story_labels',
        'pm_story_links',
        'pm_story_owners',
        'pm_story_templates',
        'story_delivery_targets',
        'story_git_links'
    ] LOOP
        legacy_relation := to_regclass(format('public.%I', legacy_table));
        IF legacy_relation IS NULL THEN
            CONTINUE;
        END IF;
        EXECUTE format('SELECT EXISTS (SELECT 1 FROM %s)', legacy_relation)
            INTO has_rows;
        IF has_rows THEN
            RAISE EXCEPTION 'task canonicalization failed: legacy table % contains rows',
                legacy_table;
        END IF;
    END LOOP;
END $$;

DROP TABLE IF EXISTS pm_story_followers;
DROP TABLE IF EXISTS pm_story_labels;
DROP TABLE IF EXISTS pm_story_owners;
DROP TABLE IF EXISTS pm_story_links;
DROP TABLE IF EXISTS pm_story_templates;
DROP TABLE IF EXISTS story_git_links;
DROP TABLE IF EXISTS story_delivery_targets;
DROP TABLE IF EXISTS pm_stories;
DROP SEQUENCE IF EXISTS pm_story_display_id_seq;

-- The old index name can survive the story_id -> task_id rename. Ensure the
-- canonical index remains available before removing the duplicate.
CREATE INDEX IF NOT EXISTS idx_agent_handoffs_task_id ON agent_handoffs (task_id);
DROP INDEX IF EXISTS idx_agent_handoffs_story_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = 'public'
           AND (column_name ~ '(^|_)story_id$' OR column_name = 'story_type')
           AND table_name <> 'schema_migrations'
    ) THEN
        RAISE EXCEPTION 'task canonicalization failed: active story-era columns remain';
    END IF;

    IF EXISTS (SELECT 1 FROM agent_runs WHERE target_type = 'story') THEN
        RAISE EXCEPTION 'task canonicalization failed: story agent targets remain';
    END IF;
    IF EXISTS (SELECT 1 FROM automation_rules WHERE trigger_type LIKE 'story.%') THEN
        RAISE EXCEPTION 'task canonicalization failed: story automation triggers remain';
    END IF;
END $$;
