-- Records remember the import that created them, so a resumed import counts
-- its own conversations as imported rather than as already present.

ALTER TABLE support_import_records
    ADD COLUMN IF NOT EXISTS job_id uuid;
