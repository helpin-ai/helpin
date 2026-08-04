import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import React from 'react';
import { ImageResponse } from 'next/og.js';

const h = React.createElement;
const websiteRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repositoryRoot = resolve(websiteRoot, '..');
const mark = await readFile(resolve(websiteRoot, 'public/brand/helpin-icon-ink-128.png'));
const markSrc = `data:image/png;base64,${mark.toString('base64')}`;

const COLORS = {
  canvas: '#f8f6f1',
  ink: '#201d19',
  muted: '#706a61',
  line: '#ded9d0',
  coral: '#e8613a',
  coralSoft: '#f7ded5',
  white: '#ffffff',
};

const variants = [
  {
    output: resolve(websiteRoot, 'public/og/helpin-home.png'),
    eyebrow: 'THE AI WORK OPERATING SYSTEM',
    headline: ['Bring every team together.', 'Put AI agents to work.'],
    support: 'Projects · Support · Sales · Docs',
    visual: 'connected',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-pricing.png'),
    eyebrow: 'HELPIN PRICING',
    headline: ['Focus on growth,', 'not the seat count.'],
    support: 'Every module. AI agents included.',
    visual: 'pricing',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-privacy.png'),
    eyebrow: 'TRUST & PRIVACY',
    headline: ['Privacy at Helpin'],
    support: 'How we protect and process your information.',
    visual: 'privacy',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-terms.png'),
    eyebrow: 'LEGAL',
    headline: ['Helpin Terms', 'of Service'],
    support: 'Clear terms for a connected workspace.',
    visual: 'terms',
  },
  {
    output: resolve(repositoryRoot, 'frontend/public/og/helpin-app.png'),
    eyebrow: 'HELPIN WORKSPACE',
    headline: ["Your team's work,", 'connected.'],
    support: 'Projects · Support · Sales · Docs · Agents',
    visual: 'app',
  },
  {
    output: resolve(repositoryRoot, 'frontend/public/og/helpin-shared-document.png'),
    eyebrow: 'SHARED VIA HELPIN',
    headline: ['Shared document'],
    support: 'Securely shared from a connected workspace.',
    visual: 'document',
  },
];

function brand() {
  return h('div', {
    style: { display: 'flex', alignItems: 'center', gap: 10 },
  },
  h('img', { src: markSrc, width: 48, height: 48, alt: '' }),
  h('div', {
    style: { display: 'flex', fontSize: 36, fontWeight: 700, letterSpacing: '-1.5px', color: COLORS.ink },
  }, 'Helpin'));
}

function connectedVisual() {
  const rows = ['Projects', 'Support', 'Sales', 'Docs'];
  return h('div', {
    style: { display: 'flex', flexDirection: 'column', justifyContent: 'center', width: 390, gap: 25 },
  }, ...rows.map((label, index) => h('div', {
    key: label,
    style: { display: 'flex', alignItems: 'center', gap: 18, marginLeft: index % 2 ? 36 : 0 },
  },
  h('div', { style: { display: 'flex', width: 118, fontSize: 25, fontWeight: 600, color: COLORS.ink } }, label),
  h('div', { style: { display: 'flex', width: 112, height: 2, background: COLORS.line } }),
  h('div', {
    style: {
      display: 'flex', alignItems: 'center', justifyContent: 'center', width: 54, height: 54,
      borderRadius: 27, background: index === 1 ? COLORS.coral : COLORS.white,
      border: `2px solid ${index === 1 ? COLORS.coral : COLORS.line}`,
      color: index === 1 ? COLORS.white : COLORS.coral, fontSize: 25, fontWeight: 700,
    },
  }, '→'))));
}

function pricingVisual() {
  return h('div', {
    style: { display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', width: 390 },
  },
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: COLORS.coral, fontWeight: 700, letterSpacing: '-2px' } }, 'NO SEAT'),
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: COLORS.coral, fontWeight: 700, letterSpacing: '-2px' } }, 'LIMITS'),
  h('div', { style: { display: 'flex', marginTop: 28, fontSize: 24, fontWeight: 700, letterSpacing: '2px', color: COLORS.ink } }, 'UNLIMITED TEAMMATES'),
  h('div', { style: { display: 'flex', marginTop: 12, fontSize: 23, color: COLORS.muted } }, 'Starter and Growth'));
}

function legalVisual(kind) {
  return h('div', {
    style: { display: 'flex', alignItems: 'center', justifyContent: 'center', width: 390, height: 360 },
  },
  h('div', {
    style: {
      display: 'flex', flexDirection: 'column', width: 230, height: 292, padding: '42px 34px', gap: 24,
      background: COLORS.white, border: `2px solid ${COLORS.line}`, borderRadius: 8,
      boxShadow: '0 20px 60px rgba(32,29,25,0.08)',
    },
  },
  h('div', { style: { display: 'flex', width: 60, height: 8, background: COLORS.coral, borderRadius: 4 } }),
  h('div', { style: { display: 'flex', width: 158, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 132, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 150, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', {
    style: {
      display: 'flex', alignItems: 'center', justifyContent: 'center', alignSelf: 'flex-end', marginTop: 18,
      width: 58, height: 58, borderRadius: 29, background: COLORS.coralSoft, color: COLORS.coral,
      fontSize: kind === 'privacy' ? 26 : 20, fontWeight: 700,
    },
  }, kind === 'privacy' ? 'OK' : 'TOS')));
}

function appVisual() {
  const rows = [
    ['Prepare launch plan', 'In progress'],
    ['Resolve priority inbox', 'Agent ready'],
    ['Follow up on renewal', 'Today'],
  ];
  return h('div', {
    style: {
      display: 'flex', width: 430, height: 340, padding: 20, gap: 18, borderRadius: 18,
      background: COLORS.ink, boxShadow: '0 28px 70px rgba(32,29,25,0.2)',
    },
  },
  h('div', { style: { display: 'flex', flexDirection: 'column', width: 72, gap: 15, paddingTop: 8 } },
    h('div', { style: { display: 'flex', width: 34, height: 34, borderRadius: 17, background: COLORS.coral } }),
    ...[1, 2, 3, 4].map((item) => h('div', { key: item, style: { display: 'flex', width: 48, height: 7, borderRadius: 4, background: '#57524c' } })),
  ),
  h('div', { style: { display: 'flex', flexDirection: 'column', flex: 1, gap: 13 } },
    h('div', { style: { display: 'flex', fontSize: 25, color: COLORS.white, fontWeight: 700, margin: '5px 0 8px' } }, 'Today'),
    ...rows.map(([title, status], index) => h('div', {
      key: title,
      style: { display: 'flex', flexDirection: 'column', padding: '17px 20px', gap: 8, borderRadius: 10, background: index === 1 ? '#3b312d' : '#302d29' },
    },
    h('div', { style: { display: 'flex', fontSize: 19, fontWeight: 600, color: COLORS.white } }, title),
    h('div', { style: { display: 'flex', fontSize: 15, color: index === 1 ? '#f39a7f' : '#aba49b' } }, status))),
  ));
}

function documentVisual() {
  return h('div', {
    style: { display: 'flex', alignItems: 'center', justifyContent: 'center', width: 390, height: 360 },
  },
  h('div', {
    style: {
      display: 'flex', flexDirection: 'column', width: 250, height: 300, padding: '44px 38px', gap: 24,
      background: COLORS.white, border: `2px solid ${COLORS.line}`, borderRadius: 12,
      boxShadow: '0 22px 70px rgba(32,29,25,0.1)',
    },
  },
  h('div', { style: { display: 'flex', width: 70, height: 10, borderRadius: 5, background: COLORS.coral } }),
  h('div', { style: { display: 'flex', width: 172, height: 7, borderRadius: 4, background: COLORS.ink } }),
  h('div', { style: { display: 'flex', width: 145, height: 6, borderRadius: 3, background: COLORS.line } }),
  h('div', { style: { display: 'flex', width: 166, height: 6, borderRadius: 3, background: COLORS.line } }),
  h('div', { style: { display: 'flex', width: 112, height: 6, borderRadius: 3, background: COLORS.line } })));
}

function visual(type) {
  if (type === 'connected') return connectedVisual();
  if (type === 'pricing') return pricingVisual();
  if (type === 'privacy' || type === 'terms') return legalVisual(type);
  if (type === 'app') return appVisual();
  return documentVisual();
}

function image(variant) {
  return h('div', {
    style: {
      display: 'flex', position: 'relative', width: '100%', height: '100%', padding: '66px 72px',
      overflow: 'hidden', color: COLORS.ink, backgroundColor: COLORS.canvas,
      backgroundImage: 'linear-gradient(rgba(32,29,25,0.035) 1px, transparent 1px), linear-gradient(90deg, rgba(32,29,25,0.035) 1px, transparent 1px)',
      backgroundSize: '32px 32px',
    },
  },
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, top: 58 },
  }, brand()),
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, right: 72, top: 148, bottom: 58, alignItems: 'center', justifyContent: 'space-between' },
  },
  h('div', { style: { display: 'flex', flexDirection: 'column', width: 650 } },
    h('div', { style: { display: 'flex', marginBottom: 22, fontSize: 17, fontWeight: 700, letterSpacing: '2.4px', color: COLORS.coral } }, variant.eyebrow),
    h('div', { style: { display: 'flex', flexDirection: 'column', fontSize: 63, lineHeight: 1.04, fontWeight: 700, letterSpacing: '-2.8px' } },
      ...variant.headline.map((line) => h('div', { key: line, style: { display: 'flex' } }, line)),
    ),
    h('div', { style: { display: 'flex', marginTop: 26, fontSize: 24, color: COLORS.muted } }, variant.support),
  ),
  visual(variant.visual)),
  h('div', { style: { display: 'flex', position: 'absolute', left: 72, bottom: 34, width: 70, height: 5, borderRadius: 3, background: COLORS.coral } }));
}

const selectedVariants = process.env.HELPIN_OG_VARIANT
  ? variants.filter((variant) => variant.output.includes(`helpin-${process.env.HELPIN_OG_VARIANT}.png`))
  : variants;

for (const variant of selectedVariants) {
  await mkdir(dirname(variant.output), { recursive: true });
  const response = new ImageResponse(image(variant), { width: 1200, height: 630 });
  await writeFile(variant.output, new Uint8Array(await response.arrayBuffer()));
  process.stdout.write(`generated ${variant.output}\n`);
}
