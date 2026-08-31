-- The verification backfill runs before application startup, so it cannot rely
-- on GORM AutoMigrate having added this model column first.
ALTER TABLE IF EXISTS support_email_routes
    ADD COLUMN IF NOT EXISTS forwarding_verified_at TIMESTAMPTZ;
