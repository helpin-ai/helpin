-- Tracks every record created by "Load sample data" so the sample workspace
-- can be removed exactly and so setup evidence and background workers can
-- ignore sample rows. entity_id references the seeded row's primary key in the
-- table named by entity_type; there is no foreign key because the entities
-- live in several tables.
CREATE TABLE IF NOT EXISTS sample_data_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sample_data_items_entity
    ON sample_data_items (entity_id);

CREATE INDEX IF NOT EXISTS idx_sample_data_items_workspace
    ON sample_data_items (workspace_id, entity_type);

-- Removing sample conversations deletes their per-user inbox state. The
-- personal counter trigger declared a variable named conversation_id, which
-- made "WHERE conversation_id = ..." ambiguous in its DELETE branch, so any
-- deletion of support_conversation_user_states rows failed (including the
-- cascade from deleting a conversation). Same logic, unambiguous names.
CREATE OR REPLACE FUNCTION support_refresh_personal_core_counters()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_conversation_id UUID := COALESCE(NEW.conversation_id, OLD.conversation_id);
    removed_user_id TEXT;
    contribution support_inbox_counter_contributions%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN
        removed_user_id := OLD.user_id::TEXT;
        FOR contribution IN
            SELECT * FROM support_inbox_counter_contributions contributions
            WHERE contributions.conversation_id = OLD.conversation_id
              AND contributions.audience_type = 'user'
              AND contributions.audience_id = removed_user_id
            FOR UPDATE
        LOOP
            PERFORM support_apply_core_counter_contribution(
                contribution.conversation_id, contribution.workspace_id,
                contribution.mailbox_scope_id, 'user', removed_user_id,
                contribution.bucket_id, 0::SMALLINT, 0::SMALLINT, 0::SMALLINT);
        END LOOP;
        DELETE FROM support_inbox_counter_contributions contributions
        WHERE contributions.conversation_id = OLD.conversation_id
          AND contributions.audience_type = 'user'
          AND contributions.audience_id = removed_user_id;
    ELSE
        PERFORM support_refresh_user_core_counters(target_conversation_id, NEW.user_id);
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$;
