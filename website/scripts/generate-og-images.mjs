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
  canvas: '#FFFFFF',
  ink: '#131514',
  muted: '#4F5A55',
  line: '#E4E9E6',
  accent: '#0F7A50',
  accentSoft: '#E7F4ED',
  white: '#ffffff',
};

const variants = [
  {
    output: resolve(websiteRoot, 'public/og/helpin-home-green-v2.png'),
    eyebrow: 'THE HELPIN PLATFORM',
    headline: ['Bring every team', 'together. Put AI', 'agents to work.'],
    support: 'Support, projects, CRM and docs. Connected by AI agents.',
    visual: 'connected',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-new-home-green-v2.png'),
    eyebrow: 'ONE CUSTOMER HISTORY',
    headline: ['AI agents that do', 'more than answer.'],
    support: 'A shared workspace for your team and AI agents.',
    visual: 'connected',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-pricing-green-v2.png'),
    eyebrow: 'HELPIN PRICING',
    headline: ['One platform.', 'Your whole team,', 'connected.'],
    support: 'Choose Helpin Cloud or self-host on your infrastructure.',
    visual: 'pricing',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-privacy-green-v2.png'),
    eyebrow: 'TRUST & PRIVACY',
    headline: ['Privacy at Helpin'],
    support: 'How we protect and process your information.',
    visual: 'privacy',
  },
  {
    output: resolve(websiteRoot, 'public/og/helpin-terms-green-v2.png'),
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
  ['customer-support', 'CUSTOMER SUPPORT', ['AI agents that', 'know the history.'], 'Support that follows through—with your team in control.'],
  ['projects', 'PROJECTS', ['Plan the work.', 'Build with AI agents', 'that know why.'], 'Roadmaps, sprints and customer history in one workspace.'],
  ['crm', 'CRM', ['Move deals forward.'], 'With AI agents that know your customers.'],
  ['meetings', 'MEETINGS', ['AI agents that turn', 'meeting decisions', 'into next steps.'], 'Keep calls, tasks and follow-ups connected to the customer.'],
  ['knowledge', 'KNOWLEDGE', ['Better docs for', 'your customers.'], 'Better answers from your AI agents.'],
  ['ai-agents', 'AI AGENTS', ['AI agents that turn', 'customer history', 'into action.'], 'Your team sets the tools, permissions and approvals.'],
  ['developers', 'FOR DEVELOPERS', ['Connect your product.'], 'Give AI agents the tools to act—with SDKs, APIs and MCP.'],
  ['self-hosting', 'OPEN SOURCE', ['The whole product.', 'Your infrastructure.'], 'One connected platform. Your team controls the deployment.'],
  ['branding', 'THE HELPIN BRAND', ['One customer history.'], 'A shared workspace for your team and AI agents.'],
];
for (const [slug, eyebrow, headline, support] of productCards) {
  variants.push({ output: resolve(websiteRoot, `public/og/helpin-${slug}-green-v2.png`), eyebrow, headline, support, visual: 'connected' });
}
// Instrument Sans: Google Fonts static TTFs, bundled for deterministic offline builds.
// Source: https://fonts.google.com/specimen/Instrument+Sans (SIL OFL in assets/instrument-sans).
const fonts = await Promise.all([400, 600, 700].map(async weight => ({
  name: 'Instrument Sans', weight, style: 'normal',
  data: await readFile(resolve(websiteRoot, `scripts/assets/instrument-sans/${weight}.ttf`)),
})));

function brand() {
  return h('div', {
    style: { display: 'flex', alignItems: 'center', gap: 10 },
  },
  h('img', { src: markSrc, width: 48, height: 48, alt: '' }),
  h('div', {
    style: { display: 'flex', fontSize: 36, fontWeight: 700, letterSpacing: '-1.5px', color: COLORS.ink },
  }, 'Helpin'));
}

const PRODUCT_STEPS = {
  'customer-support': ['CUSTOMER SUPPORT', [['Customer history', 'Earlier messages and linked work'], ['An informed answer', 'Your team and agents investigate'], ['A useful follow-up', 'Keep the customer informed']]],
  projects: ['PLAN TO DELIVERY', [['Plan the work', 'Roadmaps, sprints and objectives'], ['Build with context', 'Requirements and customer history'], ['Review and deliver', 'Keep the next step in view']]],
  crm: ['THE CUSTOMER RELATIONSHIP', [['Know the account', 'Conversations and linked work'], ['Review the deal', 'Owners, stages and next steps'], ['Follow through', 'An update grounded in history']]],
  meetings: ['AFTER THE CONVERSATION', [['Capture the call', 'Return to what was said'], ['Review decisions', 'Commitments and open questions'], ['Prepare next steps', 'Tasks and follow-ups to review']]],
  knowledge: ['KNOWLEDGE THAT HELPS', [['Publish useful guides', 'Help articles and product docs'], ['Find the answer', 'Search and source-linked answers'], ['Review an update', 'Keep guidance close to the product']]],
  'ai-agents': ['SPECIALISTS, CONNECTED', [['Ask Agent', 'Start with a question or a task'], ['Specialist agents', 'Investigate, plan and prepare'], ['Your controls', 'Selected tools and approvals']]],
  developers: ['CONNECT YOUR PRODUCT', [['SDKs and APIs', 'Connect support and customer data'], ['MCP connections', 'Give agents selected tools'], ['Events and workflows', 'Start the work you configure']]],
  'self-hosting': ['ON YOUR INFRASTRUCTURE', [['The whole product', 'Support, projects, CRM and more'], ['Your deployment', 'Install and inspect with the CLI'], ['Your connections', 'Choose supported providers']]],
};

function connectedVisual(variant) {
  const slug = variant.output.split('helpin-').at(-1).replace('-green-v2.png', '');
  const [label, steps] = PRODUCT_STEPS[slug] ?? ['ONE CUSTOMER HISTORY', [['Customer question', 'Docs, history and connected tools'], ['A task. A fix.', 'Your team and agents at work'], ['Customer follow-up', 'The conversation stays attached']]];
  return h('div', { style: { display: 'flex', flexDirection: 'column', width: 360, flexShrink: 0, padding: '28px', borderRadius: 20, background: '#0B2119', border: '1px solid #365B47' } },
    h('div', { style: { display: 'flex', color: '#9CDBB3', fontSize: 14, letterSpacing: '2px', marginBottom: 22 } }, label),
    ...steps.map(([title, detail], index) => h('div', { key: title, style: { display: 'flex', flexDirection: 'column', padding: '19px 18px', marginTop: index ? 12 : 0, borderRadius: 10, background: '#193A2B', border: '1px solid #365B47' } },
      h('div', { style: { display: 'flex', gap: 12, alignItems: 'center', color: '#EDF5EF', fontSize: 21, fontWeight: 600 } }, h('span', {style: {color: '#9CDBB3', fontSize: 13}}, String(index + 1).padStart(2, '0')), title),
      h('div', { style: { display: 'flex', color: '#B2C6BA', fontSize: 14, marginTop: 8 } }, detail))))
}

function pricingVisual() {
  return h('div', {
    style: { display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', width: 360, flexShrink: 0 },
  },
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: COLORS.accent, fontWeight: 700, letterSpacing: '-2px' } }, 'NO SEAT'),
  h('div', { style: { display: 'flex', fontSize: 72, lineHeight: 0.95, color: COLORS.accent, fontWeight: 700, letterSpacing: '-2px' } }, 'LIMITS'),
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
  h('div', { style: { display: 'flex', width: 60, height: 8, background: COLORS.accent, borderRadius: 4 } }),
  h('div', { style: { display: 'flex', width: 158, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 132, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', { style: { display: 'flex', width: 150, height: 5, background: COLORS.line, borderRadius: 3 } }),
  h('div', {
    style: {
      display: 'flex', alignItems: 'center', justifyContent: 'center', alignSelf: 'flex-end', marginTop: 18,
      width: 58, height: 58, borderRadius: 29, background: COLORS.accentSoft, color: COLORS.accent,
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
  if (type === 'pricing') return pricingVisual();
  if (type === 'privacy' || type === 'terms') return legalVisual(type);
  if (type === 'app') return appVisual();
  return documentVisual();
}

function image(variant) {
  return h('div', {
    style: {
      display: 'flex', position: 'relative', width: '100%', height: '100%', padding: '66px 72px',
      overflow: 'hidden', color: COLORS.ink, fontFamily: 'Instrument Sans', backgroundColor: COLORS.canvas,
      backgroundImage: 'linear-gradient(rgba(15,122,80,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(15,122,80,0.04) 1px, transparent 1px)',
      backgroundSize: '32px 32px',
    },
  },
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, top: 58 },
  }, brand()),
  h('div', {
    style: { display: 'flex', position: 'absolute', left: 72, right: 72, top: 148, bottom: 58, alignItems: 'center', justifyContent: 'space-between' },
  },
  h('div', { style: { display: 'flex', flexDirection: 'column', width: 620, flexShrink: 0 } },
    h('div', { style: { display: 'flex', marginBottom: 22, fontSize: 17, fontWeight: 700, letterSpacing: '2.4px', color: COLORS.accent } }, variant.eyebrow),
    h('div', { style: { display: 'flex', flexDirection: 'column', fontSize: 56, lineHeight: 1.08, fontWeight: 600, letterSpacing: '-1.8px' } },
      ...variant.headline.map((line) => h('div', { key: line, style: { display: 'flex' } }, line)),
    ),
    h('div', { style: { display: 'flex', marginTop: 26, fontSize: 21, lineHeight: 1.5, color: COLORS.muted } }, variant.support),
  ),
  visual(variant.visual, variant)),
  h('div', { style: { display: 'flex', position: 'absolute', left: 72, bottom: 34, width: 70, height: 5, borderRadius: 3, background: COLORS.accent } }));
}

const selectedVariants = process.env.HELPIN_OG_VARIANT
  ? variants.filter((variant) => variant.output.includes(`helpin-${process.env.HELPIN_OG_VARIANT}.png`) || variant.output.includes(`helpin-${process.env.HELPIN_OG_VARIANT}-green-v2.png`))
  : process.env.HELPIN_OG_WEBSITE_ONLY ? variants.filter(variant => variant.output.startsWith(websiteRoot)) : variants;

for (const variant of selectedVariants) {
  await mkdir(dirname(variant.output), { recursive: true });
  const response = new ImageResponse(image(variant), { width: 1200, height: 630, fonts });
  await writeFile(variant.output, new Uint8Array(await response.arrayBuffer()));
  process.stdout.write(`generated ${variant.output}\n`);
}
