DO $$
BEGIN
    IF to_regclass('public.coding_session_state_snapshots') IS NOT NULL THEN
        ALTER TABLE coding_session_state_snapshots
            ADD COLUMN IF NOT EXISTS through_sequence bigint NOT NULL DEFAULT 0;

        UPDATE coding_session_state_snapshots
        SET through_sequence = CASE
            WHEN jsonb_typeof(snapshot_payload -> 'through_sequence') = 'number'
                THEN (snapshot_payload ->> 'through_sequence')::bigint
            ELSE 0
        END
        WHERE through_sequence = 0;
    END IF;
END $$;
