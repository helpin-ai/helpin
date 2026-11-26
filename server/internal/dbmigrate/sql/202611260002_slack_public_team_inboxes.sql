ALTER TABLE slack_channel_routes
    ADD COLUMN IF NOT EXISTS public_team_inboxes boolean NOT NULL DEFAULT false;
