import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { inflateSync } from 'node:zlib';

const { createPageMetadata, PAGE_SEO } = await import('../src/lib/metadata.ts');
const marketingRoutes = ['/product', '/products/customer-support', '/products/projects', '/products/crm', '/products/meetings', '/products/knowledge', '/products/ai-agents', '/developers', '/self-hosting', '/branding'];

function decodeRgbaPng(png) {
  const idat = [];
  let offset = 8;
  while (offset < png.length) {
    const length = png.readUInt32BE(offset);
    const type = png.toString('ascii', offset + 4, offset + 8);
    const data = png.subarray(offset + 8, offset + 8 + length);
    if (type === 'IDAT') idat.push(data);
    offset += length + 12;
  }

  const width = png.readUInt32BE(16);
  const height = png.readUInt32BE(20);
  assert.equal(png[24], 8, 'PNG must use 8-bit channels');
  assert.equal(png[25], 6, 'PNG must use RGBA color');
  const source = inflateSync(Buffer.concat(idat));
  const stride = width * 4;
  const pixels = Buffer.alloc(stride * height);
  let sourceOffset = 0;

  const paeth = (left, up, upperLeft) => {
    const estimate = left + up - upperLeft;
    const leftDistance = Math.abs(estimate - left);
    const upDistance = Math.abs(estimate - up);
    const upperLeftDistance = Math.abs(estimate - upperLeft);
    if (leftDistance <= upDistance && leftDistance <= upperLeftDistance) return left;
    return upDistance <= upperLeftDistance ? up : upperLeft;
  };

  for (let y = 0; y < height; y += 1) {
    const filter = source[sourceOffset];
    sourceOffset += 1;
    for (let x = 0; x < stride; x += 1) {
      const raw = source[sourceOffset];
      sourceOffset += 1;
      const target = y * stride + x;
      const left = x >= 4 ? pixels[target - 4] : 0;
      const up = y > 0 ? pixels[target - stride] : 0;
      const upperLeft = y > 0 && x >= 4 ? pixels[target - stride - 4] : 0;
      const predictor = filter === 1
        ? left
        : filter === 2
          ? up
          : filter === 3
            ? Math.floor((left + up) / 2)
            : filter === 4
              ? paeth(left, up, upperLeft)
              : 0;
      pixels[target] = (raw + predictor) & 0xff;
    }
  }

  return { width, height, pixels };
}

describe('website SEO metadata', () => {
  it('defines a complete, unique contract for every public route', () => {
    const expectedRoutes = Object.keys(PAGE_SEO);
    assert.deepEqual(
      Object.values(PAGE_SEO).map(page => page.canonicalPath).sort(),
      ['/', '/pricing', '/privacy', '/terms', '/compare', ...marketingRoutes].sort(),
    );

    const titles = new Set();
    const descriptions = new Set();
    for (const route of expectedRoutes) {
      const page = PAGE_SEO[route];
      const metadata = createPageMetadata(page);

      assert.ok(page.title.length > 10 && page.title.length <= 60, `${page.title} should fit in search results`);
      assert.ok(page.description.length >= 50 && page.description.length <= 160);
      assert.match(page.canonicalPath, /^\/[a-z0-9/-]*$/);
      assert.ok(existsSync(new URL('../public' + page.imagePath, import.meta.url)));
      assert.match(page.imagePath, /^\/og\/helpin-[a-z0-9-]+\.png$/);
      assert.match(page.imageAlt, /Helpin/);
      assert.equal(metadata.alternates?.canonical, page.canonicalPath);
      assert.equal(metadata.openGraph?.url, page.canonicalPath);
      assert.equal(metadata.openGraph?.siteName, 'Helpin');
      assert.equal(metadata.openGraph?.locale, 'en_US');
      assert.equal(metadata.openGraph?.type, 'website');
      assert.deepEqual(metadata.openGraph?.images, [{
        url: page.imagePath,
        width: 1200,
        height: 630,
        type: 'image/png',
        alt: page.imageAlt,
      }]);
      assert.equal(metadata.twitter?.card, 'summary_large_image');
      assert.deepEqual(metadata.twitter?.images, [{ url: page.imagePath, alt: page.imageAlt }]);
      assert.deepEqual(metadata.robots, {
        index: true,
        follow: true,
        googleBot: {
          index: true,
          follow: true,
          'max-image-preview': 'large',
          'max-snippet': -1,
          'max-video-preview': -1,
        },
      });
      assert.equal('keywords' in metadata, false);
      assert.equal(titles.has(page.title), false);
      titles.add(page.title);
      assert.equal(descriptions.has(page.description), false, `${route} needs its own description`);
      descriptions.add(page.description);
    }
  });

  it('lists every public page in the sitemap and points robots.txt at it', async () => {
    const { default: sitemap } = await import('../src/app/sitemap.ts');
    const { default: robots } = await import('../src/app/robots.ts');
    const { COMPETITORS } = await import('../src/app/(site)/compare/compare-data.ts');
    const { ALTERNATIVES } = await import('../src/app/(site)/compare/alternatives-data.ts');
    const urls = sitemap().map(entry => entry.url);
    const pages = Object.values(PAGE_SEO).map(page => page.canonicalPath === '/' ? 'https://helpin.ai' : 'https://helpin.ai' + page.canonicalPath);
    assert.deepEqual(urls, [...pages, ...[...COMPETITORS, ...ALTERNATIVES].map(item => `https://helpin.ai/compare/${item.slug}`)]);
    assert.equal(robots().sitemap, 'https://helpin.ai/sitemap.xml');
  });

  it('ships social images as optimized 1200 by 630 PNG files', () => {
    const images = [
      '../public/og/helpin-new-home-green-v4.png',
      '../public/og/helpin-pricing-green-v4.png',
      '../public/og/helpin-privacy-green-v4.png',
      '../public/og/helpin-terms-green-v4.png',
      ...marketingRoutes.map(path => '../public/og/helpin-' + path.split('/').at(-1) + '-green-v4.png'),
      '../../frontend/public/og/helpin-app.png',
      '../../frontend/public/og/helpin-shared-document.png',
    ];

    for (const relativePath of images) {
      const fileUrl = new URL(relativePath, import.meta.url);
      assert.equal(existsSync(fileUrl), true, `${relativePath} should exist`);
      const png = readFileSync(fileUrl);
      assert.deepEqual([...png.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10]);
      assert.equal(png.readUInt32BE(16), 1200);
      assert.equal(png.readUInt32BE(20), 630);
      assert.ok(png.byteLength < 1_000_000, `${relativePath} should stay below 1 MB`);

      const { width, pixels } = decodeRgbaPng(png);
      // The mark area must contain both dark and light pixels: a mark drawn against its
      // background, whether the card uses the light or the dark theme.
      let darkPixels = 0;
      let lightPixels = 0;
      for (let y = 76; y < 104; y += 1) {
        for (let x = 78; x < 116; x += 1) {
          const pixel = (y * width + x) * 4;
          if (pixels[pixel + 3] <= 200) continue;
          if (pixels[pixel] < 70 && pixels[pixel + 1] < 70 && pixels[pixel + 2] < 70) darkPixels += 1;
          if (pixels[pixel] > 200 && pixels[pixel + 1] > 200 && pixels[pixel + 2] > 200) lightPixels += 1;
        }
      }
      assert.ok(darkPixels > 100 && lightPixels > 100, `${relativePath} should render the Helpin mark`);
    }
  });

  it('gives every comparison page complete, unique, sourced metadata', async () => {
    const { COMPETITORS, competitorSeo } = await import('../src/app/(site)/compare/compare-data.ts');
    const { ALTERNATIVES, alternativesSeo, TOOLS } = await import('../src/app/(site)/compare/alternatives-data.ts');
    const titles = new Set(Object.values(PAGE_SEO).map(page => page.title));
    for (const list of ALTERNATIVES) {
      const page = alternativesSeo(list);
      assert.ok(page.title.length <= 60, `${page.title} should fit in search results`);
      assert.ok(page.description.length >= 50 && page.description.length <= 160, `${list.slug} description length`);
      assert.equal(titles.has(page.title), false);
      titles.add(page.title);
      assert.equal(list.tools[0], 'helpin', 'Helpin is listed first and labeled as ours');
      for (const key of list.tools) assert.ok(TOOLS[key] && TOOLS[key].limits.length >= 2, `${key} needs limits`);
      assert.ok(existsSync(new URL('../public' + page.imagePath, import.meta.url)), `${page.imagePath} should exist`);
    }
    for (const competitor of COMPETITORS) {
      const page = competitorSeo(competitor);
      assert.ok(page.title.length <= 60, `${page.title} should fit in search results`);
      assert.ok(page.description.length >= 50 && page.description.length <= 160, `${competitor.slug} description length`);
      assert.equal(titles.has(page.title), false);
      titles.add(page.title);
      assert.match(competitor.checked, /^\d{4}-\d{2}-\d{2}$/);
      assert.ok(competitor.sources.length >= 3, `${competitor.slug} needs sources`);
      assert.ok(competitor.strengths.length >= 3, `${competitor.slug} should say where the competitor is stronger`);
      assert.ok(existsSync(new URL('../public' + page.imagePath, import.meta.url)), `${page.imagePath} should exist`);
    }
  });
});
