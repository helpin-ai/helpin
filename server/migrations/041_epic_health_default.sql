ALTER TABLE pm_epics
  ALTER COLUMN health SET DEFAULT 'no_health';

UPDATE pm_epics
SET health = 'no_health'
WHERE health = '';
