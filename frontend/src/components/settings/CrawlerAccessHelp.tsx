export function CrawlerAccessHelp() {
  return (
    <details className="border-t border-border/70 pt-4 text-sm">
      <summary className="cursor-pointer font-medium">Allow Helpin to crawl your website</summary>
      <div className="mt-3 space-y-3 text-muted-foreground">
        <p>Publish these rules in your site’s <code>/robots.txt</code> to allow public pages under <code>/docs/</code>. Replace that path with the public section you want to import, and set Website URL to a page in that section.</p>
        <pre className="overflow-x-auto rounded-md bg-muted/40 p-3 text-xs text-foreground">{'User-agent: Helpin-Crawler\nAllow: /docs/\nDisallow: /\n\nUser-agent: CloudflareBrowserRenderingCrawler\nAllow: /docs/\nDisallow: /'}</pre>
        <p>Helpin-Crawler is used for local crawling. CloudflareBrowserRenderingCrawler is used when your installation uses Cloudflare crawling. Update an existing group for the same crawler instead of adding conflicting rules.</p>
        <p>Only allow pages you intend to use as AI knowledge. Keep private areas blocked. For an entirely public site, use <code>Allow: /</code> without <code>Disallow: /</code>. Robots rules do not replace login protection.</p>
        <p>If a firewall or bot challenge blocks requests, allow the configured crawler for those public paths too. Then re-sync the source. Disallowed URLs are skipped; an unreachable robots.txt stops the local sync so existing content is preserved.</p>
      </div>
    </details>
  );
}
