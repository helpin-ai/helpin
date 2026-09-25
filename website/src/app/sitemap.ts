import type { MetadataRoute } from 'next';
import { PAGE_SEO, SITE_URL } from '../lib/metadata.ts';

export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
  return Object.values(PAGE_SEO).map(page => ({
    url: page.canonicalPath === '/' ? SITE_URL : `${SITE_URL}${page.canonicalPath}`,
  }));
}
