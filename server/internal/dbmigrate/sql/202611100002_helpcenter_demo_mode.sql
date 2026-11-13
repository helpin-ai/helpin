ALTER TABLE docs_helpcenter_configs
 ADD COLUMN demo_mode boolean NOT NULL DEFAULT false,
 ADD COLUMN search_indexing_enabled boolean NOT NULL DEFAULT true,
 ADD COLUMN official_docs_url text NOT NULL DEFAULT '';
