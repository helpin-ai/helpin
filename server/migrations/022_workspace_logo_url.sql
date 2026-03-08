-- Add logo_url column to workspaces table
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS logo_url TEXT;
