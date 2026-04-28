-- The destructive cluster rebuild is performed by:
--   go run ./cmd/migrate cluster-rebuild
--
-- This migration is intentionally a marker so dbmigrate records that the
-- rollout reached the rebuild step. The command is idempotent and safe to
-- run before or after this marker is applied.
SELECT 1;
