-- Migration: add_fetch_and_crawl_tools_for_competitive_intel
-- Existing template-created competitive intel agents copied allowed_tools and
-- system_prompt at creation time, so they need an explicit backfill.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_templates' AND column_name = 'allowed_tools'
    ) THEN
        UPDATE agent_templates
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["fetch_url"]'::jsonb
        WHERE key = 'competitive_intelligence_digest'
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["fetch_url"]'::jsonb);

        UPDATE agent_templates
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["crawl_url"]'::jsonb
        WHERE key = 'competitive_intelligence_digest'
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["crawl_url"]'::jsonb);
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'allowed_tools'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_template_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'skills'
    ) THEN
        UPDATE agents
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["fetch_url"]'::jsonb
        WHERE (source_template_key = 'competitive_intelligence_digest'
               OR COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb)
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["fetch_url"]'::jsonb);

        UPDATE agents
        SET allowed_tools = COALESCE(allowed_tools, '[]'::jsonb) || '["crawl_url"]'::jsonb
        WHERE (source_template_key = 'competitive_intelligence_digest'
               OR COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb)
          AND NOT (COALESCE(allowed_tools, '[]'::jsonb) @> '["crawl_url"]'::jsonb);
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'system_prompt'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'source_template_key'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = 'skills'
    ) THEN
        UPDATE agents
        SET system_prompt = trim(BOTH FROM COALESCE(system_prompt, '')) || E'\n\nSource verification:\nFor each competitor, first use web_search_exa to find official changelog, release notes, product updates, blog, docs, or roadmap pages. Then use fetch_url on exact source URLs to verify page content and dates. If search is thin, use crawl_url on the competitor''s official website or docs host with changelog/update keywords before marking no_public_changelog.'
        WHERE (source_template_key = 'competitive_intelligence_digest'
               OR COALESCE(skills, '[]'::jsonb) @> '[{"key":"competitive_intelligence_digest"}]'::jsonb)
          AND COALESCE(system_prompt, '') NOT ILIKE '%fetch_url%'
          AND COALESCE(system_prompt, '') NOT ILIKE '%crawl_url%';
    END IF;
END $$;
