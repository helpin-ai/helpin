-- Typed custom fields: workspaces define a key's label, type and options for
-- CRM records and support conversations. Values stay JSON on the records.

CREATE TABLE IF NOT EXISTS custom_field_definitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    object_type text NOT NULL,
    key text NOT NULL,
    label text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    field_type text NOT NULL,
    options jsonb NOT NULL DEFAULT '[]',
    position integer NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_field_definitions_key
    ON custom_field_definitions (workspace_id, object_type, key);

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS custom_fields jsonb NOT NULL DEFAULT '{}';
