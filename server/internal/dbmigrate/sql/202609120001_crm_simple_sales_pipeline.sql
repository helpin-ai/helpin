-- Simplify the original default in existing workspaces. Customized pipelines
-- (including changed names, stage order, probabilities or commercial motion)
-- are deliberately preserved. New workspaces use the matching repository seed.
-- Run atomically: move references before deleting merged stages. Do not emit
-- deal events, alter explicit deal probabilities, or refresh deal activity dates.
LOCK TABLE crm_pipelines, crm_pipeline_stages, crm_deals, crm_suggestions, automation_rules
    IN SHARE ROW EXCLUSIVE MODE;

DO $$
DECLARE
    pipeline record;
    stage_ids uuid[];
    merged_id uuid;
BEGIN
    FOR pipeline IN
        SELECT p.id, p.workspace_id
        FROM crm_pipelines p JOIN crm_pipeline_stages s ON s.pipeline_id = p.id
        WHERE p.is_default AND p.name = 'Sales Pipeline'
          AND p.default_commercial_motion = 'new_business'
        GROUP BY p.id, p.workspace_id
        HAVING array_agg(s.name ORDER BY s.position) = ARRAY[
            'Appointment Scheduled', 'Qualified to Buy', 'Presentation Scheduled',
            'Decision Maker Bought-In', 'Contract Sent', 'Closed Won', 'Closed Lost']::text[]
           AND array_agg(s.stage_type ORDER BY s.position) = ARRAY['open','open','open','open','open','won','lost']::text[]
           AND array_agg(s.position ORDER BY s.position) = ARRAY[0,1,2,3,4,5,6]
           AND array_agg(s.probability ORDER BY s.position) = ARRAY[20,40,60,80,90,100,0]
    LOOP
        SELECT array_agg(id ORDER BY position) INTO stage_ids
        FROM crm_pipeline_stages WHERE pipeline_id = pipeline.id;

        -- Existing, unreviewed advice was generated against the old stages.
        -- Supersede it rather than silently changing the action the user approves.
        UPDATE crm_suggestions SET status = 'superseded', updated_at = now()
        WHERE workspace_id = pipeline.workspace_id AND status = 'pending'
          AND suggestion_type IN ('deal_create', 'deal_advance')
          AND EXISTS (SELECT 1 FROM unnest(stage_ids) stage_id
                      WHERE strpos(context::text, stage_id::text) > 0);

        -- Qualification, presentation and buyer discussions share one stage.
        FOREACH merged_id IN ARRAY stage_ids[3:4] LOOP
            UPDATE crm_deals SET stage_id = stage_ids[2] WHERE stage_id = merged_id;
            -- Configurations can nest stage IDs in conditions or action payloads.
            -- Match complete JSON string values, retaining all other settings.
            UPDATE automation_rules
            SET trigger_config = replace(trigger_config::text, to_json(merged_id::text)::text, to_json(stage_ids[2]::text)::text)::jsonb,
                action_config = replace(action_config::text, to_json(merged_id::text)::text, to_json(stage_ids[2]::text)::text)::jsonb
            WHERE workspace_id = pipeline.workspace_id
              AND (strpos(trigger_config::text, merged_id::text) > 0
                   OR strpos(action_config::text, merged_id::text) > 0);
            DELETE FROM crm_pipeline_stages WHERE id = merged_id;
        END LOOP;

        -- Keep the remaining IDs stable, including won/lost outcome IDs.
        UPDATE crm_pipeline_stages s
        SET name = v.name, position = v.position, probability = v.probability
        FROM (VALUES
            (stage_ids[1], 'Lead', 0, 20),
            (stage_ids[2], 'In Discussion', 1, 50),
            (stage_ids[5], 'Proposal Sent', 2, 80),
            (stage_ids[6], 'Won', 3, 100),
            (stage_ids[7], 'Lost', 4, 0)
        ) AS v(id, name, position, probability)
        WHERE s.id = v.id;
    END LOOP;
END $$;
