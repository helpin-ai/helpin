-- Add website_url column to workspaces table
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS website_url TEXT;
