-- Rename the competitive intelligence flow/template identity while preserving
-- existing agent and automation references.

DO $$
DECLARE
    old_key CONSTANT text := 'competitive_intelligence_digest';
    new_key CONSTANT text := 'competitors_changelog_tracking_report';
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_templates' AND column_name = 'key'
    ) THEN
        UPDATE agent_templates t
        SET key = new_key
        WHERE key = old_key
          AND NOT EXISTS (
              SELECT 1
              FROM agent_templates existing
              WHERE existing.key = new_key
                AND COALESCE(existing.workspace_id::text, '') = COALESCE(t.workspace_id::text, '')
                AND existing.deleted_at IS NULL
          );

        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'agent_templates' AND column_name = 'deleted_at'
        ) THEN
            UPDATE agent_templates
            SET deleted_at = COALESCE(deleted_at, NOW()),
                is_enabled = false
            WHERE key = old_key
              AND deleted_at IS NULL;
        END IF;

        UPDATE agent_templates
        SET name = 'Competitors Changelog Tracking Report',
            description = 'Tracks recent competitor changelog updates and creates a recurring tracking report task.',
            default_role = 'Competitors Changelog Analyst'
        WHERE key = new_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_template_key'
    ) THEN
        UPDATE agents
        SET source_template_key = new_key
        WHERE source_template_key = old_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'template_key'
    ) THEN
        UPDATE agents
        SET template_key = new_key
        WHERE template_key = old_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'automation_rules' AND column_name = 'template_key'
    ) THEN
        UPDATE automation_rules
        SET template_key = new_key
        WHERE template_key = old_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'docs_documents' AND column_name = 'template_key'
    ) THEN
        UPDATE docs_documents
        SET template_key = new_key
        WHERE template_key = old_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_templates' AND column_name = 'skills'
    ) THEN
        UPDATE agent_templates
        SET skills = COALESCE((
            SELECT jsonb_agg(
                CASE
                    WHEN item->>'key' = old_key THEN jsonb_set(item, '{key}', to_jsonb(new_key), false)
                    ELSE item
                END
            )
            FROM jsonb_array_elements(COALESCE(skills, '[]'::jsonb)) AS item
        ), '[]'::jsonb)
        WHERE COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'skills'
    ) THEN
        UPDATE agents
        SET skills = COALESCE((
            SELECT jsonb_agg(
                CASE
                    WHEN item->>'key' = old_key THEN jsonb_set(item, '{key}', to_jsonb(new_key), false)
                    ELSE item
                END
            )
            FROM jsonb_array_elements(COALESCE(skills, '[]'::jsonb)) AS item
        ), '[]'::jsonb)
        WHERE COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_versions' AND column_name = 'skills'
    ) THEN
        UPDATE agent_versions
        SET skills = COALESCE((
            SELECT jsonb_agg(
                CASE
                    WHEN item->>'key' = old_key THEN jsonb_set(item, '{key}', to_jsonb(new_key), false)
                    ELSE item
                END
            )
            FROM jsonb_array_elements(COALESCE(skills, '[]'::jsonb)) AS item
        ), '[]'::jsonb)
        WHERE COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb;
    END IF;
END $$;
