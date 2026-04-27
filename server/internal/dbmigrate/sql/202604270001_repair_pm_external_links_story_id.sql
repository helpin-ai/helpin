-- Repair legacy story_id drift left behind on pm_external_links after the
-- story -> task hard cut. Some databases have both task_id and story_id, with
-- story_id still NOT NULL, which breaks task_id-based writes.

DO $$
BEGIN
    IF to_regclass('public.pm_external_links') IS NOT NULL THEN
        IF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'pm_external_links'
              AND column_name = 'story_id'
        ) AND EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'pm_external_links'
              AND column_name = 'task_id'
        ) THEN
            UPDATE pm_external_links
            SET task_id = story_id
            WHERE task_id IS NULL
              AND story_id IS NOT NULL;

            ALTER TABLE pm_external_links DROP COLUMN story_id;
        ELSIF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'pm_external_links'
              AND column_name = 'story_id'
        ) AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'pm_external_links'
              AND column_name = 'task_id'
        ) THEN
            ALTER TABLE pm_external_links RENAME COLUMN story_id TO task_id;
        END IF;
    END IF;
END $$;
