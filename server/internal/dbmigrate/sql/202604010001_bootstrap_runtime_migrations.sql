-- Bootstrap the runtime migration ledger.
-- This migration is intentionally a no-op beyond being recorded in
-- schema_migrations so future cutover migrations have a stable execution path.
SELECT 1;
