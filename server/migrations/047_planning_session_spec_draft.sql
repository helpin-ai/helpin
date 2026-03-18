-- Replace spec_sections array with a single spec_draft text column.
ALTER TABLE planning_sessions ADD COLUMN IF NOT EXISTS spec_draft TEXT NOT NULL DEFAULT '';
