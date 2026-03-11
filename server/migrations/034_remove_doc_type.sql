-- Remove the doc_type column from docs_documents.
-- External publishability is now determined by the space type (external_capable).
ALTER TABLE docs_documents DROP COLUMN IF EXISTS doc_type;
DROP INDEX IF EXISTS idx_docs_doc_ws_type_status;
