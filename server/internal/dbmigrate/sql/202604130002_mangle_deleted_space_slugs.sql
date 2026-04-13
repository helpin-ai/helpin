-- Migration: mangle_deleted_space_slugs
-- Mangle slugs of soft-deleted docs spaces so the unique constraint
-- (workspace_id, slug) is freed up for new spaces with the same name.

UPDATE docs_spaces
SET slug = slug || '-deleted-' || EXTRACT(EPOCH FROM deleted_at)::bigint
WHERE deleted_at IS NOT NULL
  AND slug NOT LIKE '%-deleted-%';
