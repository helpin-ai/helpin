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

// Website cards use a quiet, monochrome dark theme (near-black, white type, green only as a small accent).
// App cards in frontend/public/og keep the light theme.
const DARK = {
  canvas: '#090909',
  ink: '#F1F4F2',
  muted: '#9AA39E',
  eyebrow: '#8A938E',
  accent: '#A9D8B8',
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
const VERSION = 'v4';

const variants = [
  {
    output: resolve(websiteRoot, `public/og/helpin-new-home-green-${VERSION}.png`),
    eyebrow: 'ONE CUSTOMER HISTORY',
    headline: ['AI agents that do', 'more than answer.'],
    support: 'Support, projects, CRM and docs on one customer history.',
    visual: 'connected',
    art: 'home',
  },
  {
    output: resolve(websiteRoot, `public/og/helpin-pricing-green-${VERSION}.png`),
    eyebrow: 'HELPIN PRICING',
    headline: ['Every module.', 'Every teammate.', 'One price.'],
    support: 'Self-host free, or let us run it with AI included.',
    visual: 'pricing',
  },
  {
    output: resolve(websiteRoot, `public/og/helpin-privacy-green-${VERSION}.png`),
    eyebrow: 'TRUST & PRIVACY',
    headline: ['Privacy at Helpin'],
    support: 'How we protect and process your information.',
    visual: 'privacy',
  },
  {
    output: resolve(websiteRoot, `public/og/helpin-terms-green-${VERSION}.png`),
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

const productCards = [
  ['product', 'THE HELPIN PRODUCT', ['Everything in Helpin,', 'attached to', 'the customer.'], 'Support, projects, CRM, meetings, docs and AI agents.'],
  ['customer-support', 'CUSTOMER SUPPORT', ['AI agents that', 'know the history.'], 'No per-seat or per-resolution fees.'],
  ['projects', 'PROJECTS', ['Plan the work.', 'Build with AI agents', 'that know why.'], 'Roadmaps, sprints and objectives in one workspace.'],
  ['crm', 'CRM', ['Every deal, with', 'the whole customer', 'history.'], 'Contacts, companies and deals, with signals and playbooks.'],
  ['meetings', 'MEETINGS', ['Meeting notes that', 'become tracked work.'], 'Meet, Zoom, Teams and Webex calls, linked to the customer.'],
  ['knowledge', 'KNOWLEDGE', ['Better docs for', 'your customers.'], 'Better answers from your AI agents.'],
  ['ai-agents', 'AI AGENTS', ['AI agents that turn', 'customer history', 'into action.'], 'Your team sets the tools, permissions and approvals.'],
  ['developers', 'FOR DEVELOPERS', ['Connect your product.'], 'Give AI agents the tools to act, with SDKs, MCP and events.'],
  ['self-hosting', 'OPEN SOURCE', ['Same product.', 'You choose who', 'runs it.'], 'Free under AGPL-3.0, no plan limits. Community 0.1 beta.'],
  ['branding', 'THE HELPIN BRAND', ['One customer history.'], 'A shared workspace for your team and AI agents.'],
  ['compare', 'COMPARE HELPIN', ['How Helpin', 'compares.'], 'Intercom, Zendesk, Help Scout, Chatwoot and Linear.'],
  ['compare-intercom', 'HELPIN VS INTERCOM', ['Helpin vs', 'Intercom'], 'Features, pricing and switching, side by side.'],
  ['compare-zendesk', 'HELPIN VS ZENDESK', ['Helpin vs', 'Zendesk'], 'Features, pricing and switching, side by side.'],
  ['compare-help-scout', 'HELPIN VS HELP SCOUT', ['Helpin vs', 'Help Scout'], 'Features, pricing and switching, side by side.'],
  ['compare-chatwoot', 'HELPIN VS CHATWOOT', ['Helpin vs', 'Chatwoot'], 'Two open-source options, side by side.'],
  ['compare-linear', 'HELPIN VS LINEAR', ['Helpin vs', 'Linear'], 'Project tracking and customer context, side by side.'],
  ['compare-intercom-alternatives', 'INTERCOM ALTERNATIVES', ['Intercom', 'alternatives'], 'Six tools compared on pricing, AI and hosting.'],
  ['compare-zendesk-alternatives', 'ZENDESK ALTERNATIVES', ['Zendesk', 'alternatives'], 'Six tools compared on pricing, AI and hosting.'],
];
for (const [slug, eyebrow, headline, support] of productCards) {
  variants.push({ output: resolve(websiteRoot, `public/og/helpin-${slug}-green-${VERSION}.png`), eyebrow, headline, support, visual: 'connected', art: slug });
}

// Page artwork: text-free panels generated once with gpt-image-2.5-sunburst and committed,
// so builds stay offline and deterministic. Prompts and steps: scripts/assets/og-art/README.md.
// A variant without an art file falls back to its drawn visual.
const ART_WIDTH = 480;
for (const variant of variants) variant.dark = variant.output.startsWith(websiteRoot);
for (const variant of variants.filter((item) => item.art)) {
  try {
    const art = await readFile(resolve(websiteRoot, `scripts/assets/og-art/${variant.art}.jpg`));
    variant.artSrc = `data:image/jpeg;base64,${art.toString('base64')}`;
  } catch {
    variant.artSrc = null;
  }
}
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

const PRODUCT_STEPS = {
  'customer-support': ['CUSTOMER SUPPORT', [['Customer history', 'Earlier messages and linked work'], ['An informed answer', 'Your team and agents investigate'], ['A useful follow-up', 'Keep the customer informed']]],
  projects: ['PLAN TO DELIVERY', [['Plan the work', 'Roadmaps, sprints and objectives'], ['Build with context', 'Requirements and customer history'], ['Review and deliver', 'Keep the next step in view']]],
  crm: ['THE CUSTOMER RELATIONSHIP', [['Know the account', 'Conversations and linked work'], ['Review the deal', 'Owners, stages and next steps'], ['Follow through', 'An update grounded in history']]],
  meetings: ['AFTER THE CONVERSATION', [['Capture the call', 'Return to what was said'], ['Review decisions', 'Commitments and open questions'], ['Prepare next steps', 'Tasks and follow-ups to review']]],
  knowledge: ['KNOWLEDGE THAT HELPS', [['Publish useful guides', 'Help articles and product docs'], ['Find the answer', 'Search and source-linked answers'], ['Review an update', 'Keep guidance close to the product']]],
  'ai-agents': ['SPECIALISTS, CONNECTED', [['Ask Agent', 'Start with a question or a task'], ['Specialist agents', 'Investigate, plan and prepare'], ['Your controls', 'Selected tools and approvals']]],
  developers: ['CONNECT YOUR PRODUCT', [['SDKs', 'Put support inside your product'], ['MCP connections', 'Give agents selected tools'], ['Events and workflows', 'Start the work you configure']]],
  'self-hosting': ['ON YOUR INFRASTRUCTURE', [['The whole product', 'Support, projects, CRM and more'], ['Your deployment', 'Install and inspect with the CLI'], ['Your connections', 'Choose supported providers']]],
};

function connectedVisual(variant) {
  const slug = variant.output.split('helpin-').at(-1).replace(`-green-${VERSION}.png`, '');
  const [label, steps] = PRODUCT_STEPS[slug] ?? ['ONE CUSTOMER HISTORY', [['Customer question', 'Docs, history and connected tools'], ['A task. A fix.', 'Your team and agents at work'], ['Customer follow-up', 'The conversation stays attached']]];
  // Neutral charcoal panel; green only on the step numbers.
  return h('div', { style: { display: 'flex', flexDirection: 'column', width: 360, flexShrink: 0, padding: '28px', borderRadius: 16, background: '#131615', border: '1px solid #2A2E2C' } },
    h('div', { style: { display: 'flex', color: DARK.eyebrow, fontSize: 14, letterSpacing: '2px', marginBottom: 22 } }, label),
    ...steps.map(([title, detail], index) => h('div', { key: title, style: { display: 'flex', flexDirection: 'column', padding: '19px 18px', marginTop: index ? 12 : 0, borderRadius: 10, background: '#1A1D1C', border: '1px solid #2A2E2C' } },
      h('div', { style: { display: 'flex', gap: 12, alignItems: 'center', color: DARK.ink, fontSize: 21, fontWeight: 600 } }, h('span', {style: {color: '#3FA27A', fontSize: 13}}, String(index + 1).padStart(2, '0')), title),
      h('div', { style: { display: 'flex', color: DARK.muted, fontSize: 14, marginTop: 8 } }, detail))))
}

function pricingVisual(dark = false) {
  const [headline, label, detail] = dark ? [DARK.ink, DARK.muted, DARK.eyebrow] : [COLORS.accent, COLORS.ink, COLORS.muted];
  return h('div', {
    style: { display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', width: 360, flexShrink: 0 },
  },
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: headline, fontWeight: 700, letterSpacing: '-2px' } }, 'NO SEAT'),
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: headline, fontWeight: 700, letterSpacing: '-2px' } }, 'LIMITS'),
  h('div', { style: { display: 'flex', marginTop: 28, fontSize: 22, fontWeight: 700, letterSpacing: '2px', color: label } }, 'UNLIMITED TEAMMATES'),
  h('div', { style: { display: 'flex', marginTop: 12, fontSize: 20, color: detail } }, 'On every plan, and free to self-host'));
}

function legalVisual(kind, dark = false) {
  const [paper, border, line, badge, badgeText] = dark
    ? ['#131615', '#2A2E2C', '#3A403D', '#1A1D1C', '#3FA27A']
    : [COLORS.white, COLORS.line, COLORS.line, COLORS.accentSoft, COLORS.accent];
  return h('div', {
    style: { display: 'flex', alignItems: 'center', justifyContent: 'center', width: 390, height: 360 },
  },
  h('div', {
    style: {
      display: 'flex', flexDirection: 'column', width: 230, height: 292, padding: '42px 34px', gap: 24,
      background: paper, border: `${dark ? 1 : 2}px solid ${border}`, borderRadius: 8,
      boxShadow: dark ? 'none' : '0 20px 60px rgba(32,29,25,0.08)',
    },
  },
  h('div', { style: { display: 'flex', width: 60, height: 8, background: COLORS.accent, borderRadius: 4 } }),
  h('div', { style: { display: 'flex', width: 158, height: 5, background: line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 132, height: 5, background: line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 150, height: 5, background: line, borderRadius: 3 } }),
  h('div', {
    style: {
      display: 'flex', alignItems: 'center', justifyContent: 'center', alignSelf: 'flex-end', marginTop: 18,
      width: 58, height: 58, borderRadius: 29, background: badge, color: badgeText,
      border: dark ? `1px solid ${border}` : 'none', fontSize: kind === 'privacy' ? 26 : 20, fontWeight: 700,
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

function visual(type, variant) {
  if (type === 'connected') return connectedVisual(variant);
  if (type === 'pricing') return pricingVisual(variant.dark);
  if (type === 'privacy' || type === 'terms') return legalVisual(type, variant.dark);
  if (type === 'app') return appVisual();
  return documentVisual();
}

function image(variant) {
  const dark = variant.dark;
  const background = dark
    ? { backgroundColor: DARK.canvas }
    : {
      backgroundColor: COLORS.canvas,
      backgroundImage: 'linear-gradient(rgba(15,122,80,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(15,122,80,0.04) 1px, transparent 1px)',
      backgroundSize: '32px 32px',
    };
  return h('div', {
    style: {
      display: 'flex', position: 'relative', width: '100%', height: '100%', padding: '66px 72px',
      overflow: 'hidden', color: dark ? DARK.ink : COLORS.ink, fontFamily: 'Instrument Sans', ...background,
    },
  },
  // The website's vortex, as a quiet static trace behind everything else.
  ...(dark ? [h('img', { src: vortexSrc, width: 1260, height: 630, alt: '', style: { position: 'absolute', left: -30, top: -40, width: 1260, height: 630, opacity: 0.5 } })] : []),
  ...(variant.artSrc ? [
    h('img', {
      src: variant.artSrc, width: ART_WIDTH, height: 630, alt: '',
      style: { position: 'absolute', right: 0, top: 0, width: ART_WIDTH, height: 630, objectFit: 'cover' },
    }),
    // Blend the artwork into the canvas so the vortex lines flow into it instead of hitting a panel edge.
    h('div', { style: { display: 'flex', position: 'absolute', right: ART_WIDTH - 140, top: 0, width: 140, height: 630, backgroundImage: `linear-gradient(90deg, ${DARK.canvas}, rgba(9,9,9,0))` } }),
  ] : []),
  // The site's hero light: a faint green glow plus soft light from above, across the whole card.
  ...(dark ? [h('div', { style: { display: 'flex', position: 'absolute', left: 0, top: 0, width: 1200, height: 630, backgroundImage: 'radial-gradient(ellipse 760px 520px at 82% 42%, rgba(55,122,86,0.20), rgba(55,122,86,0)), radial-gradient(ellipse 900px 420px at 30% -10%, rgba(255,255,255,0.05), rgba(255,255,255,0))' } })] : []),
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, top: 58 },
  }, brand(dark)),
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, right: variant.artSrc ? ART_WIDTH + 24 : 72, top: 148, bottom: 58, alignItems: 'center', justifyContent: 'space-between' },
  },
  h('div', { style: { display: 'flex', flexDirection: 'column', width: variant.artSrc ? 600 : 620, flexShrink: 0 } },
    h('div', { style: { display: 'flex', marginBottom: 22, fontSize: 17, fontWeight: 700, letterSpacing: '2.4px', color: dark ? DARK.eyebrow : COLORS.accent } }, variant.eyebrow),
    h('div', { style: { display: 'flex', flexDirection: 'column', fontSize: 56, lineHeight: 1.08, fontWeight: 600, letterSpacing: '-1.8px' } },
      // As on the site's dark heroes, the closing part of a multi-line headline is set in mint.
      ...variant.headline.map((line, index) => h('div', { key: line, style: { display: 'flex', color: dark && variant.headline.length > 1 && index === variant.headline.length - 1 ? DARK.accent : undefined } }, line)),
    ),
    h('div', { style: { display: 'flex', marginTop: 26, fontSize: 21, lineHeight: 1.5, color: dark ? DARK.muted : COLORS.muted } }, variant.support),
  ),
  ...(variant.artSrc ? [] : [visual(variant.visual, variant)])),
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
