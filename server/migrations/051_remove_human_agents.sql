BEGIN;

CREATE TEMP TABLE IF NOT EXISTS tmp_legacy_human_agents (
    id uuid PRIMARY KEY
) ON COMMIT DROP;

TRUNCATE tmp_legacy_human_agents;

DO $$
DECLARE
    has_agent_kind boolean;
BEGIN
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'agents'
          AND column_name = 'agent_kind'
    ) INTO has_agent_kind;

    IF has_agent_kind THEN
        EXECUTE $sql$
            INSERT INTO tmp_legacy_human_agents (id)
            SELECT id
            FROM agents
            WHERE agent_class = 'human' OR agent_kind = 'human'
            ON CONFLICT (id) DO NOTHING
        $sql$;

        -- Keep the deprecated column rollout-safe for older readers until a later cleanup migration.
        EXECUTE $sql$
            UPDATE agents
            SET agent_kind = 'llm'
            WHERE COALESCE(agent_kind, '') <> 'llm'
              AND id NOT IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    ELSE
        EXECUTE $sql$
            INSERT INTO tmp_legacy_human_agents (id)
            SELECT id
            FROM agents
            WHERE agent_class = 'human'
            ON CONFLICT (id) DO NOTHING
        $sql$;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'agents'
          AND column_name = 'user_id'
    ) THEN
        EXECUTE $sql$
            UPDATE agents
            SET user_id = NULL
            WHERE user_id IS NOT NULL
        $sql$;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.pm_stories') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_stories' AND column_name = 'assigned_agent_id'
    ) THEN
        EXECUTE $sql$
            UPDATE pm_stories
            SET assigned_agent_id = NULL
            WHERE assigned_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.support_conversations') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'support_conversations' AND column_name = 'assigned_agent_id'
    ) THEN
        EXECUTE $sql$
            UPDATE support_conversations
            SET assigned_agent_id = NULL
            WHERE assigned_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.pm_epics') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_epics' AND column_name = 'orchestrator_agent_id'
    ) THEN
        EXECUTE $sql$
            UPDATE pm_epics
            SET orchestrator_agent_id = NULL
            WHERE orchestrator_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.flow_node_runs') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'flow_node_runs' AND column_name = 'agent_id'
    ) THEN
        EXECUTE $sql$
            UPDATE flow_node_runs
            SET agent_id = NULL
            WHERE agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.support_messages') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'support_messages' AND column_name = 'sender_agent_id'
    ) THEN
        EXECUTE $sql$
            UPDATE support_messages
            SET sender_agent_id = NULL
            WHERE sender_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.agent_handoffs') IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'agent_handoffs' AND column_name = 'from_agent_id'
        ) THEN
            EXECUTE $sql$
                UPDATE agent_handoffs
                SET from_agent_id = NULL
                WHERE from_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
            $sql$;
        END IF;

        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = 'agent_handoffs' AND column_name = 'to_agent_id'
        ) THEN
            EXECUTE $sql$
                UPDATE agent_handoffs
                SET to_agent_id = NULL
                WHERE to_agent_id IN (SELECT id FROM tmp_legacy_human_agents)
            $sql$;
        END IF;
    END IF;

    IF to_regclass('public.support_widget_installations') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'support_widget_installations' AND column_name = 'settings'
    ) THEN
        EXECUTE $sql$
            UPDATE support_widget_installations
            SET settings = jsonb_set(settings, '{ai_agent_id}', 'null'::jsonb, true)
            WHERE settings ? 'ai_agent_id'
              AND settings->>'ai_agent_id' IN (SELECT id::text FROM tmp_legacy_human_agents)
        $sql$;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('public.agent_run_artifacts') IS NOT NULL
       AND to_regclass('public.agent_runs') IS NOT NULL THEN
        EXECUTE $sql$
            DELETE FROM agent_run_artifacts
            WHERE run_id IN (
                SELECT id
                FROM agent_runs
                WHERE agent_id IN (SELECT id FROM tmp_legacy_human_agents)
            )
        $sql$;
    END IF;

    IF to_regclass('public.agent_runs') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_runs' AND column_name = 'agent_id'
    ) THEN
        EXECUTE $sql$
            DELETE FROM agent_runs
            WHERE agent_id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;

    IF to_regclass('public.agents') IS NOT NULL THEN
        EXECUTE $sql$
            DELETE FROM agents
            WHERE id IN (SELECT id FROM tmp_legacy_human_agents)
        $sql$;
    END IF;
END $$;

COMMIT;
