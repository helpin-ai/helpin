-- Carry forward explicit channel choices from the formerly combined activity row.
UPDATE notification_preferences
SET channel_preferences = jsonb_set(channel_preferences, '{automation.failures}', channel_preferences -> 'automation.activity')
WHERE jsonb_typeof(channel_preferences -> 'automation.activity') = 'object'
  AND NOT (channel_preferences ? 'automation.failures');

UPDATE slack_members
SET notify_categories = jsonb_set(notify_categories, '{automation.failures}', notify_categories -> 'automation.activity')
WHERE jsonb_typeof(notify_categories -> 'automation.activity') = 'boolean'
  AND NOT (notify_categories ? 'automation.failures');
