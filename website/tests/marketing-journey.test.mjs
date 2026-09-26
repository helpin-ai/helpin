import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';

const read = path => readFileSync(new URL(path, import.meta.url), 'utf8');
const files = {
  home: read('../src/app/(site)/page.tsx'),
  nav: read('../src/app/(site)/_components/PreviewNav.tsx'),
  footer: read('../src/app/(site)/_components/PreviewFooter.tsx'),
  productsMenu: read('../src/app/(site)/_components/ProductsMenu.tsx'),
  actions: read('../src/app/(site)/_components/ui.tsx'),
  pricing: read('../src/app/pricing/page.tsx'),
  redirects: read('../public/_redirects'),
};

const marketingCopy = Object.values(files).join('\n');

describe('marketing launch journey', () => {
  it('uses the new homepage and its current conversion paths', () => {
    assert.match(files.home, /AI agents that do more than answer\./);
    assert.match(files.home, /<CtaRow primaryLabel="Start free trial"/);
    assert.match(files.actions, /https:\/\/app\.helpin\.ai\/register/);
    assert.match(files.actions, /https:\/\/cal\.com\/helpin-ai\/30min/);
    assert.match(files.pricing, /Try Growth free for 14 days\./);
    assert.doesNotMatch(marketingCopy, /Get early access|Free forever/i);
  });

  it('links navigation and footer to the public product routes', () => {
    assert.match(files.nav, /href="\/developers"/);
    assert.match(files.nav, /href="\/pricing"/);
    assert.match(files.footer, /href: '\/products\/customer-support'/);
    assert.match(files.footer, /href: '\/products\/ai-agents'/);
    assert.match(files.productsMenu, /\/products\/projects/);
    assert.doesNotMatch(marketingCopy, /href=["']\/new(?:["'#?]|\/(?:products|developers|self-hosting|branding)(?:[\/"'#?]|$))/);
  });

  it('redirects former preview pages while keeping asset URLs available', () => {
    for (const rule of ['/new / 301', '/new/product /product 301', '/new/products/* /products/:splat 301']) {
      assert.ok(files.redirects.includes(rule), `${rule} should exist`);
    }
    assert.doesNotMatch(files.redirects, /^\/new\/\*\s/m);
  });
});
