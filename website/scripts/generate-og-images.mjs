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
const markWhite = await readFile(resolve(websiteRoot, 'public/brand/helpin-icon-white-128.png'));
const markWhiteSrc = `data:image/png;base64,${markWhite.toString('base64')}`;

// Website cards follow the marketing site: near-black, white type, and restrained mint accents.
// App cards in frontend/public/og keep the light theme.
const DARK = {
  canvas: '#090909',
  ink: '#EDF3EF',
  accent: '#9CDBB3',
};

// A static trace of the site's HeroVortex "flow" lines (same formula and gradient as
// src/app/(site)/_components/HeroVortex.tsx), so the cards carry the website's signature.
const FLOW_LINES = [0, 1].flatMap((group) => Array.from({ length: 12 }, (_, i) => ({
  group,
  d: `M -120 ${130 + group * 250 + i * 13} C 330 ${-80 + group * 370 + i * 17}, 810 ${530 - group * 220 + i * 9}, 1520 ${150 + group * 260 + i * 12}`,
  opacity: 0.22 + (1 - Math.abs(i - 5.5) / 6) * 0.3,
})));
const vortexSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1400 700" width="1400" height="700">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop stop-color="#0B7A4E"/><stop offset="50%" stop-color="#0F9D63"/><stop offset="100%" stop-color="#3FC48F"/></linearGradient></defs>
${[0, 1].map((group) => `<g transform="rotate(${group ? -7 : 7} 700 350)" fill="none" stroke="url(#g)" stroke-linecap="round">
<g stroke-width="4" opacity="0.22">${FLOW_LINES.filter((l) => l.group === group).map((l) => `<path d="${l.d}" opacity="${l.opacity.toFixed(3)}"/>`).join('')}</g>
<g stroke-width="1">${FLOW_LINES.filter((l) => l.group === group).map((l) => `<path d="${l.d}" opacity="${l.opacity.toFixed(3)}"/>`).join('')}</g></g>`).join('')}
</svg>`;
const vortexSrc = `data:image/svg+xml;base64,${Buffer.from(vortexSvg).toString('base64')}`;

const COLORS = {
  canvas: '#FFFFFF',
  ink: '#131514',
  muted: '#4F5A55',
  line: '#E4E9E6',
  accent: '#0F7A50',
  accentSoft: '#E7F4ED',
  white: '#ffffff',
};

// Bump when card content changes so social platforms fetch the new image instead of a cached one.
// Update the matching paths in src/lib/metadata.ts and src/app/(site)/compare/compare-data.ts.
const VERSION = 'v6';

// One short headline per website card. Explicit lines keep type large at feed size.
const websiteCards = [
  ['new-home', ['AI agents for', 'support, docs, sales,', 'and coding.']],
  ['product', ['Your team. Your agents.', 'Working together.']],
  ['customer-support', ['AI agents for', 'customer support.']],
  ['projects', ['Plan the work.', 'Ship with AI agents.']],
  ['crm', ['AI agents for', 'sales follow-ups.']],
  ['meetings', ['AI meeting notes.', 'Clear next steps.']],
  ['knowledge', ['Docs that keep up', 'with your product.']],
  ['ai-agents', ['AI agents that', 'get work done.']],
  ['developers', ['Connect your tools.', 'Put agents to work.']],
  ['self-hosting', ['Your team. Your agents.', 'Your servers.']],
  ['branding', ['The Helpin brand.']],
  ['pricing', ['Your team. Your agents.', 'One workspace.']],
  ['privacy', ['Privacy at Helpin.']],
  ['terms', ['Terms of service.']],
  ['compare', ['Find your fit.', 'Compare Helpin.']],
  ['compare-intercom', ['Helpin vs', 'Intercom']],
  ['compare-zendesk', ['Helpin vs', 'Zendesk']],
  ['compare-help-scout', ['Helpin vs', 'Help Scout']],
  ['compare-chatwoot', ['Helpin vs', 'Chatwoot']],
  ['compare-chatbase', ['Helpin vs', 'Chatbase']],
  ['compare-crisp', ['Helpin vs', 'Crisp']],
  ['compare-linear', ['Helpin vs', 'Linear']],
  ['compare-plane', ['Helpin vs', 'Plane']],
  ['compare-jira', ['Helpin vs', 'Jira']],
];
const variants = websiteCards.map(([slug, headline]) => ({
  output: resolve(websiteRoot, `public/og/helpin-${slug}-green-${VERSION}.png`),
  headline,
  dark: true,
}));

// App and shared-document cards keep their existing light presentation.
variants.push(
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
);

// Instrument Sans: Google Fonts static TTFs, bundled for deterministic offline builds.
// Source: https://fonts.google.com/specimen/Instrument+Sans (SIL OFL in assets/instrument-sans).
const fonts = await Promise.all([400, 600, 700].map(async weight => ({
  name: 'Instrument Sans', weight, style: 'normal',
  data: await readFile(resolve(websiteRoot, `scripts/assets/instrument-sans/${weight}.ttf`)),
})));

function brand(dark) {
  return h('div', {
    style: { display: 'flex', alignItems: 'center', gap: 10 },
  },
  h('img', { src: dark ? markWhiteSrc : markSrc, width: 48, height: 48, alt: '' }),
  h('div', {
    style: { display: 'flex', fontSize: 36, fontWeight: 700, letterSpacing: '-1.5px', color: dark ? DARK.ink : COLORS.ink },
  }, 'Helpin'));
}

function websiteImage(variant) {
  return h('div', {
    style: {
      display: 'flex', position: 'relative', width: '100%', height: '100%',
      overflow: 'hidden', backgroundColor: DARK.canvas, color: DARK.ink,
      fontFamily: 'Instrument Sans',
    },
  },
  h('div', {
    style: {
      display: 'flex', position: 'absolute', left: 0, top: 0, width: 1200, height: 630,
      backgroundImage: 'radial-gradient(ellipse 900px 680px at 100% 100%, rgba(15,122,80,0.22), rgba(15,122,80,0))',
    },
  }),
  h('img', {
    src: vortexSrc, width: 1260, height: 630, alt: '',
    style: { position: 'absolute', left: -30, top: 0, width: 1260, height: 630, opacity: 0.22 },
  }),
  h('div', { style: { display: 'flex', position: 'absolute', left: 72, top: 58, alignItems: 'center', gap: 12 } },
    h('img', { src: markWhiteSrc, width: 64, height: 64, alt: '' }),
    h('div', { style: { display: 'flex', fontSize: 48, fontWeight: 700, letterSpacing: '-2px' } }, 'Helpin'),
  ),
  h('div', {
    style: {
      display: 'flex', position: 'absolute', left: 72, right: 72, top: 186, bottom: 70,
      flexDirection: 'column', justifyContent: 'center',
      fontSize: 88, lineHeight: 1.08, fontWeight: 600, letterSpacing: '-2.5px',
    },
  }, ...variant.headline.map((line, index) => h('div', {
    key: line,
    style: {
      display: 'flex', whiteSpace: 'nowrap',
      color: index > 0 ? DARK.accent : DARK.ink,
    },
  }, line))));
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
    h('div', { style: { display: 'flex', width: 34, height: 34, borderRadius: 17, background: COLORS.accent } }),
    ...[1, 2, 3, 4].map((item) => h('div', { key: item, style: { display: 'flex', width: 48, height: 7, borderRadius: 4, background: '#365B47' } })),
  ),
  h('div', { style: { display: 'flex', flexDirection: 'column', flex: 1, gap: 13 } },
    h('div', { style: { display: 'flex', fontSize: 25, color: COLORS.white, fontWeight: 700, margin: '5px 0 8px' } }, 'Today'),
    ...rows.map(([title, status], index) => h('div', {
      key: title,
      style: { display: 'flex', flexDirection: 'column', padding: '17px 20px', gap: 8, borderRadius: 10, background: index === 1 ? '#193A2B' : '#112D20' },
    },
    h('div', { style: { display: 'flex', fontSize: 19, fontWeight: 600, color: COLORS.white } }, title),
    h('div', { style: { display: 'flex', fontSize: 15, color: index === 1 ? '#9CDBB3' : '#B2C6BA' } }, status))),
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
  h('div', { style: { display: 'flex', width: 70, height: 10, borderRadius: 5, background: COLORS.accent } }),
  h('div', { style: { display: 'flex', width: 172, height: 7, borderRadius: 4, background: COLORS.ink } }),
  h('div', { style: { display: 'flex', width: 145, height: 6, borderRadius: 3, background: COLORS.line } }),
  h('div', { style: { display: 'flex', width: 166, height: 6, borderRadius: 3, background: COLORS.line } }),
  h('div', { style: { display: 'flex', width: 112, height: 6, borderRadius: 3, background: COLORS.line } })));
}

function image(variant) {
  if (variant.dark) return websiteImage(variant);
  return h('div', {
    style: {
      display: 'flex', position: 'relative', width: '100%', height: '100%', padding: '66px 72px',
      overflow: 'hidden', color: COLORS.ink, fontFamily: 'Instrument Sans',
      backgroundColor: COLORS.canvas,
      backgroundImage: 'linear-gradient(rgba(15,122,80,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(15,122,80,0.04) 1px, transparent 1px)',
      backgroundSize: '32px 32px',
    },
  },
  h('div', { style: { display: 'flex', position: 'absolute', left: 72, top: 58 } }, brand(false)),
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, right: 72, top: 148, bottom: 58, alignItems: 'center', justifyContent: 'space-between' },
  },
  h('div', { style: { display: 'flex', flexDirection: 'column', width: 620, flexShrink: 0 } },
    h('div', { style: { display: 'flex', marginBottom: 22, fontSize: 17, fontWeight: 700, letterSpacing: '2.4px', color: COLORS.accent } }, variant.eyebrow),
    h('div', { style: { display: 'flex', flexDirection: 'column', fontSize: 56, lineHeight: 1.08, fontWeight: 600, letterSpacing: '-1.8px' } },
      ...variant.headline.map((line) => h('div', { key: line, style: { display: 'flex' } }, line)),
    ),
    h('div', { style: { display: 'flex', marginTop: 26, fontSize: 21, lineHeight: 1.5, color: COLORS.muted } }, variant.support),
  ), variant.visual === 'app' ? appVisual() : documentVisual()),
  h('div', { style: { display: 'flex', position: 'absolute', left: 72, bottom: 34, width: 70, height: 5, borderRadius: 3, background: COLORS.accent } }));
}

const selectedVariants = process.env.HELPIN_OG_VARIANT
  ? variants.filter((variant) => variant.output.includes(`helpin-${process.env.HELPIN_OG_VARIANT}.png`) || variant.output.includes(`helpin-${process.env.HELPIN_OG_VARIANT}-green-${VERSION}.png`))
  : process.env.HELPIN_OG_WEBSITE_ONLY ? variants.filter(variant => variant.output.startsWith(websiteRoot)) : variants;

for (const variant of selectedVariants) {
  await mkdir(dirname(variant.output), { recursive: true });
  const response = new ImageResponse(image(variant), { width: 1200, height: 630, fonts });
  await writeFile(variant.output, new Uint8Array(await response.arrayBuffer()));
  process.stdout.write(`generated ${variant.output}\n`);
}
