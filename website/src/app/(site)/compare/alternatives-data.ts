// "Best X alternatives" list pages. Tool facts are shared across lists and come from each
// vendor's public pricing and docs on CHECKED. Helpin is listed first and labeled as ours.
import type { PageSeo } from '../../../lib/metadata.ts';

export type Tool = {
  key: string;
  name: string;
  bestFor: string;
  summary: string;
  startingPrice: string;
  priceModel: string;
  aiPricing: string;
  selfHost: string;
  strengths: string[];
  limits: string[];
  /** Our comparison page, when we have one. */
  compare?: string;
};

export type AlternativesPage = {
  slug: string;
  competitor: string;
  checked: string;
  seo: { title: string; description: string };
  lede: string;
  reasons: { title: string; body: string }[];
  criteria: { title: string; body: string }[];
  tools: string[];
  faqs: (readonly [question: string, answer: string])[];
};

const CHECKED = '2026-09-26';

export const TOOLS: Record<string, Tool> = {
  helpin: {
    key: 'helpin',
    name: 'Helpin',
    bestFor: 'Teams whose support questions turn into product work',
    summary: 'Open-source support, projects, CRM, meetings and docs on one customer history, with AI agents that can plan the fix and follow up when it ships.',
    startingPrice: '$79 a month per workspace, billed annually',
    priceModel: 'Per workspace, unlimited teammates',
    aiPricing: 'Usage allowance included; optional metered overage',
    selfHost: 'Yes, free (AGPL-3.0, 0.1 beta)',
    strengths: [
      'Support, projects, CRM, meetings and docs in one product',
      'Coding agents open pull requests from the customer conversation',
      'One price per workspace, with AI usage included',
      'Open source: Helpin Cloud or your own servers',
    ],
    limits: [
      'Web chat and email only; no phone, WhatsApp or social channels',
      'No native mobile SDKs, SSO or SLA policies yet',
      'A younger product; importers for Intercom and Zendesk are coming soon',
    ],
  },
  intercom: {
    key: 'intercom',
    name: 'Intercom',
    bestFor: 'AI-first support across many channels',
    summary: 'A mature support platform built around the Fin AI agent, with a broad channel mix and a large app ecosystem.',
    startingPrice: '$29 per seat a month, billed annually',
    priceModel: 'Per seat',
    aiPricing: 'Fin at $0.99 per outcome',
    selfHost: 'No',
    strengths: ['Phone, WhatsApp, SMS, social and Slack in one inbox', 'More than 450 apps and native mobile SDKs', 'Proactive messaging, SLAs and SSO on higher plans'],
    limits: ['Costs grow with seats, AI outcomes and add-ons', 'Project work and deals live in other tools', 'Hosted only'],
    compare: '/compare/intercom',
  },
  zendesk: {
    key: 'zendesk',
    name: 'Zendesk',
    bestFor: 'Large, multichannel service operations',
    summary: 'An established help desk with ticketing, a native contact center and deep admin controls.',
    startingPrice: '$55 per agent a month for Suite Team, billed annually',
    priceModel: 'Per agent',
    aiPricing: 'Per verified resolution beyond a small allowance; price not published',
    selfHost: 'No',
    strengths: ['Voice, IVR and messaging channels', 'Enterprise controls: SSO, SLAs, sandboxes, custom roles', 'More than 1,800 marketplace apps'],
    limits: ['Per-agent pricing, with Copilot at $50 per agent', 'Zendesk Sell (CRM) retires in August 2027', 'Hosted only'],
    compare: '/compare/zendesk',
  },
  helpscout: {
    key: 'helpscout',
    name: 'Help Scout',
    bestFor: 'Small teams that want a simple, email-like inbox',
    summary: 'A calm shared inbox and docs product, with a free plan for up to five users.',
    startingPrice: 'Free for up to 5 users; $25 per user a month on Standard, billed annually',
    priceModel: 'Per user',
    aiPricing: 'AI Answers at $0.75 per resolution',
    selfHost: 'No',
    strengths: ['Easy to learn and pleasant to use', 'WhatsApp, Instagram, Messenger and SMS on paid plans', 'Built-in importer from Zendesk, Intercom and 30+ tools'],
    limits: ['Projects and CRM through integrations', 'AI resolutions billed on top of users', 'Hosted only'],
    compare: '/compare/help-scout',
  },
  freshdesk: {
    key: 'freshdesk',
    name: 'Freshdesk',
    bestFor: 'Small and mid-size teams that want affordable ticketing',
    summary: 'Freshworks’ help desk, with email and social ticketing, and Freshdesk Omni for chat, WhatsApp and more.',
    startingPrice: '$19 per agent a month on Growth, billed annually',
    priceModel: 'Per agent',
    aiPricing: 'Freddy AI Agent: 500 sessions to start, then $49 per 100',
    selfHost: 'No',
    strengths: ['Low entry price and quick setup', 'More than 1,000 marketplace apps', 'SLAs and automation from the Growth plan'],
    limits: ['Chat and messaging need Freshdesk Omni', 'Copilot is an add-on on Pro and Enterprise', 'Hosted only'],
  },
  front: {
    key: 'front',
    name: 'Front',
    bestFor: 'B2B teams sharing email inboxes across support, sales and accounts',
    summary: 'A shared inbox that feels like email, with rules, internal comments and an AI agent called Autopilot.',
    startingPrice: '$25 per seat a month on Starter, billed annually',
    priceModel: 'Per seat',
    aiPricing: 'Autopilot from $0.05 per conversation; Copilot $20 per seat',
    selfHost: 'No',
    strengths: ['Email-style collaboration with comments and assignments', 'Email, SMS, chat, WhatsApp and social channels', 'More than 100 integrations and an API'],
    limits: ['Per-seat pricing, with AI features as add-ons', 'Advanced reporting from the Professional plan', 'Hosted only'],
  },
  chatwoot: {
    key: 'chatwoot',
    name: 'Chatwoot',
    bestFor: 'An open-source, multichannel inbox',
    summary: 'A widely used open-source support inbox with many channels and a paid enterprise edition.',
    startingPrice: 'Free to self-host; Cloud from $19 per agent a month',
    priceModel: 'Per agent',
    aiPricing: 'Captain AI credits; $20 per 1,000 extra',
    selfHost: 'Yes (MIT core; some features paid)',
    strengths: ['WhatsApp, social, Telegram, LINE, SMS and voice', 'Mature project with a large community', 'Docker, Kubernetes and marketplace installs'],
    limits: ['Captain AI, SSO and SLAs need a paid edition', 'No deals, meeting notes or project planning', 'Self-hosting means running and upgrading it yourself'],
    compare: '/compare/chatwoot',
  },
};

export const ALTERNATIVES: AlternativesPage[] = [
  {
    slug: 'intercom-alternatives',
    competitor: 'Intercom',
    checked: CHECKED,
    seo: {
      title: 'Best Intercom Alternatives: 6 Tools Compared',
      description: 'Six Intercom alternatives compared on pricing, AI costs, channels and self-hosting: Helpin, Zendesk, Help Scout, Freshdesk, Front and Chatwoot.',
    },
    lede: 'Why teams look beyond Intercom, what to compare, and six alternatives with their prices, strengths and limits. We make Helpin, so it’s listed first, and we say where the others fit better.',
    reasons: [
      { title: 'AI costs that grow with volume', body: 'Fin is billed at $0.99 per outcome, on top of seats. As AI handles more conversations, the bill rises with it.' },
      { title: 'Per-seat pricing', body: 'Each full teammate is a paid seat, which makes it costly to bring engineering, sales and success into the same view.' },
      { title: 'Work that leaves the tool', body: 'Bugs, feature requests and deals move to Jira or a CRM, and the customer context has to be copied across.' },
      { title: 'No self-hosted option', body: 'Intercom is hosted only, which rules it out for teams that must keep customer data on their own infrastructure.' },
    ],
    criteria: [
      { title: 'Price model', body: 'Per seat, per agent or per workspace, and what happens to the bill as the team grows.' },
      { title: 'How AI is billed', body: 'Per resolution, per session, credits, or an included allowance, and whether you can predict it.' },
      { title: 'Channels', body: 'Whether you need phone, WhatsApp and social, or mainly chat and email.' },
      { title: 'What happens after the reply', body: 'Whether bugs, projects and deals stay connected to the conversation.' },
      { title: 'Hosting and data', body: 'Cloud only, or an open-source edition you can run yourself.' },
      { title: 'Switching', body: 'What you can export from Intercom and what each tool can import.' },
    ],
    tools: ['helpin', 'zendesk', 'helpscout', 'freshdesk', 'front', 'chatwoot'],
    faqs: [
      ['What is the best Intercom alternative?', 'It depends on what you need beyond the inbox. Zendesk suits large multichannel teams, Help Scout and Freshdesk suit smaller budgets, Front suits shared email across teams, and Helpin suits teams whose support turns into product work.'],
      ['What is the cheapest Intercom alternative?', 'Chatwoot and Helpin are free to self-host. On Cloud, Help Scout has a free plan for up to five users, and Freshdesk starts at $19 per agent a month billed annually. Helpin charges one price per workspace, so it gets cheaper per person as the team grows.'],
      ['Is there an open-source Intercom alternative?', 'Yes. Helpin is open source under AGPL-3.0, and Chatwoot has an MIT-licensed core with a paid enterprise edition. Both can run on your own servers.'],
      ['Which Intercom alternatives include AI in the price?', 'Helpin includes an AI usage allowance in every Cloud plan. Zendesk, Help Scout, Freshdesk, Front and Chatwoot bill AI separately, per resolution, session, conversation or credit.'],
      ['Can I move my data from Intercom?', 'Intercom exports conversations as CSV or through its API. Help Scout has a built-in importer from Intercom. Helpin’s Intercom importer is coming soon, and you can run Helpin alongside Intercom in the meantime.'],
    ],
  },
  {
    slug: 'zendesk-alternatives',
    competitor: 'Zendesk',
    checked: CHECKED,
    seo: {
      title: 'Best Zendesk Alternatives: 6 Tools Compared',
      description: 'Six Zendesk alternatives compared on pricing, AI costs, channels and self-hosting: Helpin, Intercom, Help Scout, Freshdesk, Front and Chatwoot.',
    },
    lede: 'Why teams look beyond Zendesk, what to compare, and six alternatives with their prices, strengths and limits. We make Helpin, so it’s listed first, and we say where the others fit better.',
    reasons: [
      { title: 'Per-agent costs and add-ons', body: 'Suite plans are priced per agent, and Copilot adds $50 per agent. AI resolutions beyond a small allowance are billed separately.' },
      { title: 'A heavy admin surface', body: 'Zendesk’s depth suits large service teams, but smaller teams often find setup and ongoing configuration heavy.' },
      { title: 'The sales CRM is retiring', body: 'Zendesk plans to retire Zendesk Sell in August 2027 and stop offering a sales CRM.' },
      { title: 'No self-hosted option', body: 'Zendesk is hosted only, which rules it out for teams that must keep customer data on their own infrastructure.' },
    ],
    criteria: [
      { title: 'Price model', body: 'Per seat, per agent or per workspace, and what happens to the bill as the team grows.' },
      { title: 'How AI is billed', body: 'Per resolution, per session, credits, or an included allowance, and whether the price is published.' },
      { title: 'Channels', body: 'Whether you need voice and IVR, or mainly chat, email and messaging.' },
      { title: 'CRM and product work', body: 'Where deals, bugs and projects live once Zendesk Sell retires.' },
      { title: 'Hosting and data', body: 'Cloud only, or an open-source edition you can run yourself.' },
      { title: 'Switching', body: 'What Zendesk lets you export and what each tool can import.' },
    ],
    tools: ['helpin', 'intercom', 'helpscout', 'freshdesk', 'front', 'chatwoot'],
    faqs: [
      ['What is the best Zendesk alternative?', 'It depends on your team. Intercom suits AI-first support across many channels, Help Scout and Freshdesk suit smaller budgets, Front suits shared email across teams, and Helpin suits teams whose support turns into product work.'],
      ['What is the cheapest Zendesk alternative?', 'Chatwoot and Helpin are free to self-host. On Cloud, Help Scout has a free plan for up to five users, and Freshdesk starts at $19 per agent a month billed annually. Helpin charges one price per workspace with unlimited teammates.'],
      ['What should Zendesk Sell users move to?', 'Zendesk recommends Pipedrive as its migration partner. If you also want support and projects on the same history, Helpin includes contacts, companies and deals.'],
      ['Which Zendesk alternatives include AI in the price?', 'Helpin includes an AI usage allowance in every Cloud plan. Intercom, Help Scout, Freshdesk, Front and Chatwoot bill AI separately, per outcome, resolution, session, conversation or credit.'],
      ['Can I move my data from Zendesk?', 'Zendesk account owners can request ticket and user exports. Help Scout has a built-in importer from Zendesk. Helpin’s Zendesk importer is coming soon, and you can run Helpin alongside Zendesk in the meantime.'],
    ],
  },
];

export function alternativesSeo(page: AlternativesPage): PageSeo {
  return {
    title: `${page.seo.title} (${page.checked.slice(0, 4)})`,
    description: page.seo.description,
    canonicalPath: `/compare/${page.slug}`,
    imagePath: `/og/helpin-compare-${page.slug}-green-v4.png`,
    imageAlt: `${page.competitor} alternatives`,
  };
}
