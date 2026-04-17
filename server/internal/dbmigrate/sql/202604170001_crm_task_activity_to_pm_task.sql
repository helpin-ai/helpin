-- Migration A: convert crm_activities rows with activity_type='task' into
-- first-class pm_tasks rows, linked to their original CRM objects via
-- crm_associations. After the backfill, the task-typed activity rows are
-- deleted — task-as-activity is retired in favor of PMTask.
--
-- Plan: docs/plans/2026-04-16-crm-pm-task-unification-plan.md
--
-- The migration has five stages:
--   1. Ensure every workspace that has task-activities also has a sales team.
--   2. Ensure every such sales team has a workflow with pipeline states.
--   3. Materialize activity→task mappings and insert pm_tasks rows.
--   4. Write crm_associations linking new tasks to their CRM objects.
--   5. Delete the migrated activity rows; add a CHECK to block future writes.
--
-- Idempotent: subsequent runs find zero task-typed activity rows (deleted) and
-- zero workspaces-needing-sales-team (already seeded), so no-op. The CHECK is
-- guarded by a NOT EXISTS on pg_constraint.

BEGIN;

-- Stage 1: seed a Sales team in every workspace that has task-activities
-- but no sales team yet. Handle is "sales" if free, else "sales-1".."sales-20".
WITH workspaces_needing_sales AS (
    SELECT DISTINCT ca.workspace_id
    FROM crm_activities ca
    WHERE ca.activity_type = 'task'
      AND NOT EXISTS (
          SELECT 1 FROM workspace_teams wt
          WHERE wt.workspace_id = ca.workspace_id
            AND wt.team_type = 'sales'
      )
),
candidate_handles AS (
    SELECT
        wns.workspace_id,
        CASE
            WHEN NOT EXISTS (
                SELECT 1 FROM workspace_teams wt2
                WHERE wt2.workspace_id = wns.workspace_id
                  AND LOWER(wt2.handle) = 'sales'
            ) THEN 'sales'
            ELSE 'sales-' || (
                SELECT MIN(n)::text
                FROM generate_series(1, 20) n
                WHERE NOT EXISTS (
                    SELECT 1 FROM workspace_teams wt3
                    WHERE wt3.workspace_id = wns.workspace_id
                      AND LOWER(wt3.handle) = 'sales-' || n
                )
            )
        END AS handle
    FROM workspaces_needing_sales wns
)
INSERT INTO workspace_teams (workspace_id, name, handle, team_type, default_task_type, sprints_enabled, docs_publisher_enabled)
SELECT workspace_id, 'Sales', handle, 'sales', 'chore', FALSE, FALSE
FROM candidate_handles
WHERE handle IS NOT NULL;

-- Stage 2: for every sales team that has no workflow yet, create a workflow
-- with the 6-state pipeline (To contact → Contacted → Meeting booked →
-- Proposal → Won → Lost). We return the (team_id, workflow_id, default_state_id)
-- tuple so stage 3 can use it.
WITH sales_teams_needing_workflow AS (
    SELECT wt.id AS team_id, wt.workspace_id, wt.name AS team_name
    FROM workspace_teams wt
    WHERE wt.team_type = 'sales'
      AND NOT EXISTS (
          SELECT 1 FROM pm_workflows pw
          WHERE pw.workspace_id = wt.workspace_id
            AND pw.team_id = wt.id
      )
      AND EXISTS (
          -- Only seed workflows for sales teams that will actually receive
          -- migrated tasks. Avoids touching workspaces that happen to have a
          -- sales team but no task-activities.
          SELECT 1 FROM crm_activities ca
          WHERE ca.workspace_id = wt.workspace_id
            AND ca.activity_type = 'task'
      )
),
inserted_workflows AS (
    INSERT INTO pm_workflows (id, workspace_id, name, team_id, auto_assign_owner, created_at, updated_at)
    SELECT gen_random_uuid(), workspace_id, team_name || ' Workflow', team_id, FALSE, NOW(), NOW()
    FROM sales_teams_needing_workflow
    RETURNING id, team_id, workspace_id
),
inserted_states AS (
    INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at)
    SELECT gen_random_uuid(), iw.id, s.name, s.state_type, s.position, s.is_default, NOW(), NOW()
    FROM inserted_workflows iw
    CROSS JOIN (
        VALUES
            ('To contact'::text,     'unstarted'::text, 0, TRUE),
            ('Contacted',            'started',         1, FALSE),
            ('Meeting booked',       'started',         2, FALSE),
            ('Proposal',             'started',         3, FALSE),
            ('Won',                  'done',            4, FALSE),
            ('Lost',                 'done',            5, FALSE)
    ) AS s(name, state_type, position, is_default)
    RETURNING id, workflow_id, is_default
)
UPDATE pm_workflows pw
SET default_state_id = ist.id
FROM inserted_states ist
WHERE ist.workflow_id = pw.id
  AND ist.is_default = TRUE;

-- Stage 3: materialize the activity → task mapping and insert pm_tasks rows.
-- Each task is placed into the sales team's default workflow state
-- ("To contact") unless the activity's occurred_at is in the past, in which
-- case we use the "Lost" state to reflect that the work is abandoned rather
-- than pending. We allocate display_ids contiguously starting one above the
-- workspace's current MAX(display_id).
WITH activity_source AS (
    SELECT
        ca.id              AS activity_id,
        ca.workspace_id    AS workspace_id,
        ca.contact_id      AS contact_id,
        ca.company_id      AS company_id,
        ca.deal_id         AS deal_id,
        ca.owner_member_id AS owner_member_id,
        ca.subject         AS subject,
        ca.body            AS body,
        ca.occurred_at     AS occurred_at,
        ca.created_at      AS created_at,
        ca.updated_at      AS updated_at,
        sales_team.id      AS team_id,
        sales_wf.id        AS workflow_id,
        default_state.id   AS default_state_id,
        done_state.id      AS done_state_id,
        COALESCE(
            (SELECT MAX(display_id) FROM pm_tasks pt WHERE pt.workspace_id = ca.workspace_id),
            0
        ) + ROW_NUMBER() OVER (
            PARTITION BY ca.workspace_id ORDER BY ca.occurred_at, ca.id
        ) AS allocated_display_id
    FROM crm_activities ca
    LEFT JOIN LATERAL (
        SELECT wt.id FROM workspace_teams wt
        WHERE wt.workspace_id = ca.workspace_id AND wt.team_type = 'sales'
        ORDER BY wt.created_at ASC LIMIT 1
    ) sales_team ON TRUE
    LEFT JOIN LATERAL (
        SELECT pw.id, pw.default_state_id FROM pm_workflows pw
        WHERE pw.workspace_id = ca.workspace_id AND pw.team_id = sales_team.id
        ORDER BY pw.created_at ASC LIMIT 1
    ) sales_wf ON TRUE
    LEFT JOIN pm_workflow_states default_state
        ON default_state.id = sales_wf.default_state_id
    LEFT JOIN LATERAL (
        SELECT pws.id FROM pm_workflow_states pws
        WHERE pws.workflow_id = sales_wf.id AND pws.name = 'Lost'
        LIMIT 1
    ) done_state ON TRUE
    WHERE ca.activity_type = 'task'
),
inserted_tasks AS (
    INSERT INTO pm_tasks (
        id, workspace_id, display_id, name, description, task_type,
        workflow_id, workflow_state_id, team_id, owner_member_id,
        priority, severity, deadline, position,
        started, completed, blocked, archived,
        created_at, updated_at
    )
    SELECT
        gen_random_uuid(),
        asrc.workspace_id,
        asrc.allocated_display_id,
        COALESCE(NULLIF(TRIM(asrc.subject), ''), LEFT(COALESCE(asrc.body, '(untitled task)'), 80)),
        asrc.body,
        'chore',
        asrc.workflow_id,
        CASE
            WHEN asrc.occurred_at < NOW() AND asrc.done_state_id IS NOT NULL
                THEN asrc.done_state_id
            ELSE asrc.default_state_id
        END,
        asrc.team_id,
        asrc.owner_member_id,
        'none',
        'none',
        asrc.occurred_at::date,
        0,
        FALSE,
        FALSE,
        FALSE,
        FALSE,
        asrc.created_at,
        asrc.updated_at
    FROM activity_source asrc
    WHERE asrc.team_id IS NOT NULL
      AND asrc.workflow_id IS NOT NULL
      AND asrc.default_state_id IS NOT NULL
    RETURNING id, workspace_id, display_id
),
task_activity_map AS (
    SELECT
        it.id            AS new_task_id,
        asrc.activity_id AS activity_id,
        asrc.workspace_id,
        asrc.contact_id,
        asrc.company_id,
        asrc.deal_id
    FROM inserted_tasks it
    JOIN activity_source asrc
      ON asrc.workspace_id = it.workspace_id
     AND asrc.allocated_display_id = it.display_id
)
-- Stage 4: link each new task to its source CRM objects.
INSERT INTO crm_associations (workspace_id, from_object_type, from_object_id, to_object_type, to_object_id)
SELECT workspace_id, 'task', new_task_id, 'contact', contact_id
FROM task_activity_map WHERE contact_id IS NOT NULL
UNION ALL
SELECT workspace_id, 'task', new_task_id, 'company', company_id
FROM task_activity_map WHERE company_id IS NOT NULL
UNION ALL
SELECT workspace_id, 'task', new_task_id, 'deal', deal_id
FROM task_activity_map WHERE deal_id IS NOT NULL;

-- Stage 5a: delete the migrated task-activity rows.
DELETE FROM crm_activities WHERE activity_type = 'task';

-- Stage 5b: prevent future task-typed activity rows. App layer also rejects.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'crm_activities_activity_type_chk'
    ) THEN
        ALTER TABLE crm_activities
            ADD CONSTRAINT crm_activities_activity_type_chk
            CHECK (activity_type IN ('note', 'call', 'meeting', 'email'))
            NOT VALID;
        ALTER TABLE crm_activities VALIDATE CONSTRAINT crm_activities_activity_type_chk;
    END IF;
END$$;

COMMIT;
