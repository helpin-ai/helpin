import type { MetadataRoute } from 'next';
import { PAGE_SEO, SITE_URL } from '../lib/metadata.ts';
import { ALTERNATIVES, alternativesSeo } from './(site)/compare/alternatives-data.ts';
import { COMPETITORS, competitorSeo } from './(site)/compare/compare-data.ts';

export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
  return [...Object.values(PAGE_SEO), ...COMPETITORS.map(competitorSeo), ...ALTERNATIVES.map(alternativesSeo)].map(page => ({
    url: page.canonicalPath === '/' ? SITE_URL : `${SITE_URL}${page.canonicalPath}`,
  }));
}
