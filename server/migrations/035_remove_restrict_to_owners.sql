-- Remove the restrict_to_owners column from docs_spaces.
-- The flag was stored but never enforced on the backend.
ALTER TABLE docs_spaces DROP COLUMN IF EXISTS restrict_to_owners;
