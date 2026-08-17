ALTER TABLE crm_meetings
    ADD COLUMN IF NOT EXISTS recording_content_type TEXT;

-- Existing canonical recordings are audio artifacts copied by the original
-- provider-neutral pipeline. Preserve them for inline legacy playback.
UPDATE crm_meetings
SET recording_content_type = CASE
    WHEN LOWER(recording_object_key) LIKE '%.mp3' THEN 'audio/mpeg'
    WHEN LOWER(recording_object_key) LIKE '%.webm' THEN 'audio/webm'
    WHEN LOWER(recording_object_key) LIKE '%.mp4' THEN 'video/mp4'
    ELSE 'application/octet-stream'
END
WHERE recording_object_key IS NOT NULL
  AND recording_content_type IS NULL;
