-- Marker migration. The actual backfill is a one-shot Go command
-- (cmd/backfill-docs-ordering). This migration exists so the
-- migrations table records that the backfill step belongs here
-- in the sequence.
SELECT 1;
