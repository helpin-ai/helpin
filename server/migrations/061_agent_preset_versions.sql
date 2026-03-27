-- Reference-only SQL.
-- The app boot path uses GORM AutoMigrate plus explicit Go schema helpers.
-- This documents the additive schema change for preset family/version support.

ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS preset_version_key text;
