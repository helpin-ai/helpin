import type { CreateSupportContentSourceRequest, SupportContentSource } from './pmTypes';

function normalizeComparableWebsiteURL(raw?: string | null): string | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = new URL(raw);
    const path = parsed.pathname.replace(/\/+$/, '');
    const pathname = path === '' || path === '/' ? '' : path;
    const search = parsed.search || '';
    return `${parsed.protocol}//${parsed.host.toLowerCase()}${pathname}${search}`;
  } catch {
    return null;
  }
}

export function findWorkspaceWebsiteContentSource(
  websiteURL: string | undefined,
  sources: SupportContentSource[],
): SupportContentSource | null {
  const target = normalizeComparableWebsiteURL(websiteURL);
  if (!target) {
    return null;
  }

  return sources.find((source) => normalizeComparableWebsiteURL(source.start_url) === target) ?? null;
}

export function buildWorkspaceWebsiteContentSourcePayload(
  workspaceName: string,
  websiteURL: string,
): CreateSupportContentSourceRequest {
  return {
    name: `${workspaceName} Website`,
    start_url: websiteURL,
    crawl_limit: 100,
    crawl_depth: 2,
    crawl_source: 'all',
    formats: ['markdown'],
    render: true,
    include_external_links: false,
    include_subdomains: false,
    include_patterns: [],
    exclude_patterns: [],
    crawl_purposes: ['search', 'ai-input'],
    max_age_seconds: 86400,
  };
}
