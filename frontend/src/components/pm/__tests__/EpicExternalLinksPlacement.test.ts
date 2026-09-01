import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));

describe('epic external links placement', () => {
  it('keeps external links in the right-rail Related section', () => {
    const source = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');
    const overviewStart = source.indexOf("{activeView === 'overview' ? (");
    const rightRailStart = source.indexOf('Right column — metadata sidebar');
    const relatedStart = source.indexOf('<details id="epic-related-section"', rightRailStart);
    const docsStart = source.indexOf('section="docs"', relatedStart);
    const externalLinksStart = source.indexOf('<ExternalLinks', relatedStart);
    const supportStart = source.indexOf('section="support"', relatedStart);
    const crmStart = source.indexOf('section="crm"', relatedStart);

    expect(rightRailStart).toBeGreaterThan(overviewStart);
    expect(relatedStart).toBeGreaterThan(rightRailStart);
    expect(docsStart).toBeGreaterThan(relatedStart);
    expect(externalLinksStart).toBeGreaterThan(docsStart);
    expect(supportStart).toBeGreaterThan(externalLinksStart);
    expect(crmStart).toBeGreaterThan(supportStart);
    expect(source.match(/<ExternalLinks/g)).toHaveLength(1);

    const externalLinksBlock = source.slice(externalLinksStart, supportStart);
    expect(externalLinksBlock).toContain('entityType="epic"');
    expect(externalLinksBlock).toContain('flat');

    const overview = source.slice(overviewStart, rightRailStart);
    expect(overview).not.toContain('<ExternalLinks');
    expect(overview).not.toContain('External Links');
    expect(overview).toContain('<Attachments');
  });

  it('removes duplicate page-level external-link visibility state and fetching', () => {
    const source = readFileSync(resolve(__dirname, '../../../pages/pm/EpicDetail.tsx'), 'utf8');

    expect(source).not.toContain('showExternalLinks');
    expect(source).not.toContain('hasExternalLinkItems');
    expect(source).not.toContain('externalLinkCount');
    expect(source).not.toContain('pmExternalLinkService');
    expect(source).not.toContain('handleExternalLinkContentChange');
    expect(source).not.toContain('handleExternalLinkCountChange');
  });
});
