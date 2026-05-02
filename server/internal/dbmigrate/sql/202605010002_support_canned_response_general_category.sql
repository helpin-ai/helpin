ALTER TABLE support_canned_responses
  ADD COLUMN IF NOT EXISTS tag TEXT NOT NULL DEFAULT 'General';

UPDATE support_canned_responses
SET tag = 'General'
WHERE tag IS NULL OR tag = '' OR tag = 'Others';

ALTER TABLE support_canned_responses
  ALTER COLUMN tag SET DEFAULT 'General';
