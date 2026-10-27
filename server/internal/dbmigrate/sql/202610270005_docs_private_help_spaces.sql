-- A help center space can be limited to customers signed in to the portal.
-- Their live articles carry customer_published_at instead of
-- public_published_at, so every public query, which requires
-- public_published_at, leaves them out.
ALTER TABLE docs_spaces ADD COLUMN IF NOT EXISTS helpcenter_audience TEXT NOT NULL DEFAULT 'public';
ALTER TABLE docs_spaces DROP CONSTRAINT IF EXISTS chk_docs_spaces_helpcenter_audience;
ALTER TABLE docs_spaces ADD CONSTRAINT chk_docs_spaces_helpcenter_audience
    CHECK (helpcenter_audience IN ('public', 'customers'));
ALTER TABLE docs_helpcenter_articles ADD COLUMN IF NOT EXISTS customer_published_at TIMESTAMPTZ;

-- Changing a space from external to internal used to leave its articles live
-- on the public paths that do not check the space type. Take them down.
DELETE FROM docs_helpcenter_search_entries e
USING docs_documents d, docs_spaces s
WHERE e.document_id = d.id AND d.space_id = s.id AND s.type <> 'external_capable';
UPDATE docs_helpcenter_articles ha
SET public_published_at = NULL
FROM docs_documents d, docs_spaces s
WHERE ha.document_id = d.id AND d.space_id = s.id
  AND s.type <> 'external_capable' AND ha.public_published_at IS NOT NULL;
