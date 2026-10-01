import { describe, expect, it } from 'vitest';
import {
  buildWebsiteSourcePayload,
  createWebsiteSourceDraft,
  draftFromWebsiteSource,
} from '../knowledgeSourceWebsiteForm';
import type { SupportContentSource } from '../pmTypes';

describe('knowledgeSourceWebsiteForm', () => {
  it('prefills a website draft with a source name based on the website URL', () => {
    const draft = createWebsiteSourceDraft({
      initialUrl: 'https://usermaven.com/docs',
      workspaceName: 'Acme',
    });

    expect(draft.name).toBe('Usermaven.com website');
    expect(draft.startUrl).toBe('https://usermaven.com/docs');
  });

  it('builds a content source payload from editable modal fields', () => {
    const payload = buildWebsiteSourcePayload({
      ...createWebsiteSourceDraft(),
      name: 'Docs',
      startUrl: ' https://docs.example.com ',
      includePatternsText: '/docs/*\n\n/help/*',
      excludePatternsText: '/blog/*',
      crawlLimit: '250',
      crawlDepth: '4',
      crawlSource: 'sitemaps',
      render: false,
      includeExternalLinks: true,
    });

    expect(payload).toMatchObject({
      name: 'Docs',
      start_url: 'https://docs.example.com',
      crawl_limit: 250,
      crawl_depth: 4,
      crawl_source: 'sitemaps',
      render: false,
      include_external_links: true,
      include_subdomains: false,
      include_patterns: ['/docs/*', '/help/*'],
      exclude_patterns: ['/blog/*'],
      formats: ['markdown'],
      crawl_purposes: ['search', 'ai-input'],
    });
  });

  it('prefills blog exclusions and lets the user remove them before creating a source', () => {
    const draft = createWebsiteSourceDraft({ initialUrl: 'https://example.com' });
    expect(buildWebsiteSourcePayload(draft).exclude_patterns).toEqual(['/blog', '/blog/*', '*://blog.*/**']);
    expect(buildWebsiteSourcePayload({ ...draft, excludePatternsText: '' }).exclude_patterns).toEqual([]);
  });

  it('creates an editable draft from an existing website source', () => {
    const source: SupportContentSource = {
      id: 'source-1',
      workspace_id: 'workspace-1',
      name: 'Marketing site',
      start_url: 'https://example.com',
      crawl_limit: 42,
      crawl_depth: 3,
      crawl_source: 'links',
      formats: ['markdown'],
      render: true,
      include_external_links: false,
      include_subdomains: true,
      include_patterns: ['/pricing'],
      exclude_patterns: ['/legal'],
      crawl_purposes: ['search', 'ai-input'],
      max_age_seconds: 86400,
      sync_status: 'ready',
      sync_progress: 100,
      indexed_pages: 12,
      indexed_chunks: 80,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    };

    expect(draftFromWebsiteSource(source)).toMatchObject({
      name: 'Marketing site',
      startUrl: 'https://example.com',
      includePatternsText: '/pricing',
      excludePatternsText: '/legal',
      crawlLimit: '42',
      crawlDepth: '3',
      crawlSource: 'links',
      includeSubdomains: true,
    });
  });
});
