-- A nullable column created by AutoMigrate is not changed by the earlier
-- ADD COLUMN IF NOT EXISTS migration. Preserve legacy semantics (revision 0).
UPDATE support_translations SET policy_revision = 0 WHERE policy_revision IS NULL;
ALTER TABLE support_translations
  ALTER COLUMN policy_revision SET DEFAULT 0,
  ALTER COLUMN policy_revision SET NOT NULL;
