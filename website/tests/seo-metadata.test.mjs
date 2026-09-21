import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { inflateSync } from 'node:zlib';

const { createPageMetadata, PAGE_SEO } = await import('../src/lib/metadata.ts');
const { previewMetadata } = await import('../src/app/new/_components/preview-metadata.ts');
const previewRoutes = ['/new', '/new/product', '/new/products/customer-support', '/new/products/projects', '/new/products/crm', '/new/products/meetings', '/new/products/knowledge', '/new/products/ai-agents', '/new/developers', '/new/self-hosting', '/new/branding'];

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
    const expectedRoutes = ['home', 'pricing', 'privacy', 'terms'];
    assert.deepEqual(Object.keys(PAGE_SEO), expectedRoutes);

    const titles = new Set();
    for (const route of expectedRoutes) {
      const page = PAGE_SEO[route];
      const metadata = createPageMetadata(page);

      assert.ok(page.title.length > 10);
      assert.ok(page.description.length >= 50 && page.description.length <= 160);
      assert.match(page.canonicalPath, /^\/(?:pricing|privacy|terms)?$/);
      assert.match(page.imagePath, /^\/og\/helpin-[a-z-]+\.png$/);
      assert.match(page.imageAlt, /Helpin/);
      assert.equal(metadata.alternates?.canonical, page.canonicalPath);
      assert.equal(metadata.openGraph?.url, page.canonicalPath);
      assert.equal(metadata.openGraph?.siteName, 'Helpin');
      assert.equal(metadata.openGraph?.locale, 'en_US');
      assert.equal(metadata.openGraph?.type, 'website');
      assert.deepEqual(metadata.openGraph?.images, [{
        url: page.imagePath,
        secureUrl: page.imagePath,
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
    }
  });

  it('gives every new page its own branded OG and Twitter image', () => {
    for (const path of previewRoutes) {
      const metadata = previewMetadata('Helpin — ' + path, path);
      const image = metadata.openGraph.images[0];
      assert.match(image.url, /-green\.png$/);
      assert.equal(metadata.twitter.images[0].url, image.url);
      assert.equal(metadata.robots.index, false);
      assert.ok(existsSync(new URL('../public' + image.url, import.meta.url)));
    }
  });

  it('ships social images as optimized 1200 by 630 PNG files', () => {
    const images = [
      '../public/og/helpin-home-green.png',
      '../public/og/helpin-pricing-green.png',
      '../public/og/helpin-privacy-green.png',
      '../public/og/helpin-terms-green.png',
      ...previewRoutes.filter(path => path !== '/new').map(path => '../public/og/helpin-' + path.split('/').at(-1) + '-green.png'),
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
      let darkBrandPixels = 0;
      for (let y = 76; y < 104; y += 1) {
        for (let x = 78; x < 116; x += 1) {
          const pixel = (y * width + x) * 4;
          if (pixels[pixel] < 70 && pixels[pixel + 1] < 70 && pixels[pixel + 2] < 70 && pixels[pixel + 3] > 200) {
            darkBrandPixels += 1;
          }
        }
      }
      assert.ok(darkBrandPixels > 100, `${relativePath} should render the Helpin mark`);
    }
  });
});
