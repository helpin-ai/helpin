-- Reconcile dual-schema environments where task-era tables were created
-- before the story -> task hard-cut rename ran.
--
-- This backfills legacy story-era rows into the task-era tables when both
-- sets exist, then removes the leftover story-era tables so the schema ends
-- in a true hard-cut task state.

DO $$
BEGIN
    IF to_regclass('public.pm_stories') IS NOT NULL
       AND to_regclass('public.pm_tasks') IS NOT NULL THEN
        INSERT INTO pm_tasks (
            id,
            workspace_id,
            display_id,
            name,
            description,
            task_type,
            workflow_id,
            workflow_state_id,
            epic_id,
            sprint_id,
            team_id,
            owner_id,
            owner_member_id,
            requester_id,
            requester_member_id,
            estimate,
            priority,
            severity,
            deadline,
            position,
            started,
            started_at,
            completed,
            completed_at,
            moved_at,
            blocked,
            blocker,
            archived,
            assigned_agent_id,
            plan_document_id,
            template_id,
            recurring_template_id,
            recurring_run_id,
            recurring_occurrence_number,
            external_id,
            slice_type,
            implementation_brief,
            created_at,
            updated_at
        )
        SELECT
            id,
            workspace_id,
            display_id,
            name,
            description,
            story_type,
            workflow_id,
            workflow_state_id,
            epic_id,
            sprint_id,
            team_id,
            owner_id,
            owner_member_id,
            requester_id,
            requester_member_id,
            estimate,
            priority,
            severity,
            deadline,
            position,
            started,
            started_at,
            completed,
            completed_at,
            moved_at,
            blocked,
            blocker,
            archived,
            assigned_agent_id,
            plan_document_id,
            template_id,
            recurring_template_id,
            recurring_run_id,
            recurring_occurrence_number,
            external_id,
            slice_type,
            implementation_brief,
            created_at,
            updated_at
        FROM pm_stories
        ON CONFLICT (id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_story_owners') IS NOT NULL
       AND to_regclass('public.pm_task_owners') IS NOT NULL THEN
        INSERT INTO pm_task_owners (task_id, user_id, created_at)
        SELECT story_id, user_id, created_at
        FROM pm_story_owners
        ON CONFLICT (task_id, user_id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_story_followers') IS NOT NULL
       AND to_regclass('public.pm_task_followers') IS NOT NULL THEN
        INSERT INTO pm_task_followers (task_id, user_id, created_at)
        SELECT story_id, user_id, created_at
        FROM pm_story_followers
        ON CONFLICT (task_id, user_id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_story_labels') IS NOT NULL
       AND to_regclass('public.pm_task_labels') IS NOT NULL THEN
        INSERT INTO pm_task_labels (task_id, label_id, created_at)
        SELECT story_id, label_id, created_at
        FROM pm_story_labels
        ON CONFLICT (task_id, label_id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_story_links') IS NOT NULL
       AND to_regclass('public.pm_task_links') IS NOT NULL THEN
        INSERT INTO pm_task_links (
            id,
            workspace_id,
            source_task_id,
            target_task_id,
            link_type,
            created_by,
            created_at,
            updated_at
        )
        SELECT
            id,
            workspace_id,
            source_story_id,
            target_story_id,
            link_type,
            created_by,
            created_at,
            updated_at
        FROM pm_story_links
        ON CONFLICT (id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_story_templates') IS NOT NULL
       AND to_regclass('public.pm_task_templates') IS NOT NULL THEN
        INSERT INTO pm_task_templates (
            id,
            workspace_id,
            team_id,
            name,
            description,
            task_type,
            priority,
            severity,
            estimate,
            label_ids,
            owner_member_id,
            epic_id,
            sprint_id,
            deadline,
            checklist_items,
            external_links,
            archived,
            created_at,
            updated_at
        )
        SELECT
            id,
            workspace_id,
            team_id,
            name,
            description,
            story_type,
            priority,
            severity,
            estimate,
            label_ids,
            owner_member_id,
            epic_id,
            sprint_id,
            deadline,
            checklist_items,
            external_links,
            archived,
            created_at,
            updated_at
        FROM pm_story_templates
        ON CONFLICT (id) DO NOTHING;
    END IF;

    IF to_regclass('public.story_delivery_targets') IS NOT NULL
       AND to_regclass('public.task_delivery_targets') IS NOT NULL THEN
        INSERT INTO task_delivery_targets (
            id,
            workspace_id,
            task_id,
            repository_id,
            repo_full_name,
            integration_id,
            base_branch,
            working_branch,
            delivery_state,
            active_pr_number,
            active_pr_title,
            active_pr_url,
            active_pr_status,
            last_commit_sha,
            last_run_id,
            last_synced_at,
            created_at,
            updated_at
        )
        SELECT
            id,
            workspace_id,
            story_id,
            repository_id,
            repo_full_name,
            integration_id,
            base_branch,
            working_branch,
            delivery_state,
            active_pr_number,
            active_pr_title,
            active_pr_url,
            active_pr_status,
            last_commit_sha,
            last_run_id,
            last_synced_at,
            created_at,
            updated_at
        FROM story_delivery_targets
        ON CONFLICT (id) DO NOTHING;
    END IF;

    IF to_regclass('public.story_git_links') IS NOT NULL
       AND to_regclass('public.task_git_links') IS NOT NULL THEN
        INSERT INTO task_git_links (
            id,
            workspace_id,
            task_id,
            integration_id,
            repository_id,
            run_id,
            provider,
            repo,
            branch,
            pr_number,
            pr_title,
            pr_url,
            pr_status,
            commit_sha,
            created_at,
            updated_at
        )
        SELECT
            id,
            workspace_id,
            story_id,
            integration_id,
            repository_id,
            run_id,
            provider,
            repo,
            branch,
            pr_number,
            pr_title,
            pr_url,
            pr_status,
            commit_sha,
            created_at,
            updated_at
        FROM story_git_links
        ON CONFLICT (id) DO NOTHING;
    END IF;

    IF to_regclass('public.pm_task_display_id_seq') IS NOT NULL
       AND to_regclass('public.pm_tasks') IS NOT NULL THEN
        PERFORM setval(
            'pm_task_display_id_seq',
            GREATEST(COALESCE((SELECT MAX(display_id) FROM pm_tasks), 0), 1),
            true
        );
    END IF;
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
