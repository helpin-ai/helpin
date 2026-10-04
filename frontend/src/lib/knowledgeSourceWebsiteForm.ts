import type { CreateSupportContentSourceRequest, SupportContentSource } from './pmTypes';

export type WebsiteSourceDraft = {
  name: string;
  startUrl: string;
  crawlLimit: string;
  crawlDepth: string;
  crawlSource: 'all' | 'sitemaps' | 'links';
  render: boolean;
  includeExternalLinks: boolean;
  includeSubdomains: boolean;
  includePatternsText: string;
  excludePatternsText: string;
};

export function createWebsiteSourceDraft({
  initialUrl = '',
  workspaceName = '',
}: {
  initialUrl?: string;
  workspaceName?: string;
} = {}): WebsiteSourceDraft {
  const suggestedName = suggestWebsiteSourceName(initialUrl);
  return {
    name: suggestedName || (workspaceName.trim() ? `${workspaceName.trim()} website` : ''),
    startUrl: initialUrl,
    crawlLimit: '100',
    crawlDepth: '2',
    crawlSource: 'all',
    render: true,
    includeExternalLinks: false,
    includeSubdomains: false,
    includePatternsText: '',
    excludePatternsText: '/blog\n/blog/*\n*://blog.*/**',
  };
}

export function draftFromWebsiteSource(source: SupportContentSource): WebsiteSourceDraft {
  return {
    name: source.name,
    startUrl: source.start_url,
    crawlLimit: String(source.crawl_limit),
    crawlDepth: String(source.crawl_depth),
    crawlSource: source.crawl_source,
    render: source.render,
    includeExternalLinks: source.include_external_links,
    includeSubdomains: source.include_subdomains,
    includePatternsText: source.include_patterns.join('\n'),
    excludePatternsText: source.exclude_patterns.join('\n'),
  };
}

export function buildWebsiteSourcePayload(draft: WebsiteSourceDraft): CreateSupportContentSourceRequest {
  return {
    name: draft.name.trim(),
    start_url: draft.startUrl.trim(),
    crawl_limit: parsePositiveInteger(draft.crawlLimit, 100),
    crawl_depth: parsePositiveInteger(draft.crawlDepth, 2),
    crawl_source: draft.crawlSource,
    formats: ['markdown'],
    render: draft.render,
    include_external_links: draft.includeExternalLinks,
    include_subdomains: draft.includeSubdomains,
    include_patterns: splitPatternText(draft.includePatternsText),
    exclude_patterns: splitPatternText(draft.excludePatternsText),
    crawl_purposes: ['search', 'ai-input'],
    max_age_seconds: 86400,
  };
}

export function suggestWebsiteSourceName(value: string): string {
  if (!value.trim()) {
    return '';
  }

  try {
    const host = new URL(value).hostname.replace(/^www\./, '');
    return `${host.charAt(0).toUpperCase()}${host.slice(1)} website`;
  } catch {
    return '';
  }
}

function splitPatternText(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

function parsePositiveInteger(value: string, fallback: number): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}
