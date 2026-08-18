DO $$
BEGIN
    IF to_regclass('public.crm_meeting_settings') IS NOT NULL THEN
        ALTER TABLE crm_meeting_settings
            ALTER COLUMN enabled SET DEFAULT TRUE,
            ALTER COLUMN bot_name SET DEFAULT 'Helpin.ai Notetaker',
            ALTER COLUMN record_audio_by_default SET DEFAULT TRUE;

        UPDATE crm_meeting_settings
        SET bot_name = 'Helpin.ai Notetaker'
        WHERE bot_name = 'Helpin Notetaker';
    END IF;
END;
$$;
