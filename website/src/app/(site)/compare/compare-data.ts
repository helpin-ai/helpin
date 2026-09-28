// Comparison page content. One entry per competitor; the /compare/[slug] template renders it.
// Competitor facts come from their public pages on the `checked` date (see `sources`).
// Helpin facts must match the shipped product and /pricing. Re-check at least quarterly,
// then update CHECKED (it also sets the year in each page title).
import type { PageSeo } from '../../../lib/metadata.ts';

export type Cell = boolean | string;
export type Status = 'yes' | 'partial' | 'no';
type FAQ = readonly [question: string, answer: string];

// Icons are resolved in ComparePage so this file stays plain data.
export type IconKey =
  | 'workflow' | 'billing' | 'team' | 'hosting' | 'crm' | 'open' | 'maturity' | 'requests' | 'agents' | 'loop'
  | 'channels' | 'ecosystem' | 'enterprise' | 'reporting' | 'messaging' | 'simplicity' | 'import' | 'mobile'
  | 'community' | 'deploy' | 'speed' | 'models' | 'docs';

export type StepStatus = 'Available now' | 'Beta' | 'Coming soon' | 'Not yet';

export type TableRow = { label: string; helpin: Cell; competitor: Cell; helpinStatus?: Status; competitorStatus?: Status };

export type CalculatorPlan = { name: string; annual: number; monthly?: number; minSeats?: number };

export type Calculator = {
  seatsLabel: string;
  /** What one seat is called in running text, singular then plural. */
  seatsUnit: readonly [string, string];
  seats: number;
  plans: CalculatorPlan[];
  /** Index of the competitor plan closest to the Helpin plan below. */
  plan: number;
  helpinPlan: 'starter' | 'growth';
  /** Per-resolution AI pricing, when the competitor publishes one. */
  ai?: { label: string; price: number; volume: number; max: number; step: number };
  /** What the estimate leaves out, shown under the result. */
  notes: string[];
  selfHosted?: { plans: CalculatorPlan[]; plan: number; note: string };
};

export type Competitor = {
  slug: string;
  name: string;
  group: 'Customer support' | 'Project management';
  category: string;
  cardLine: string;
  checked: string;
  /** Title without the year; competitorSeo adds the year from `checked`. */
  seo: { title: string; description: string };
  /** The H1 reads "Helpin vs {name}: {accent}". */
  hero: { accent: string; lede: string };
  /** Three short rows for the hero's comparison cards: where the products differ most. */
  glance: { label: string; competitor: string; helpin: string }[];
  summary: { title: string; lede: string; competitor: string[]; helpin: string[] };
  /** Common reasons teams look beyond the competitor, each with Helpin's approach. */
  /** Omit competitorLane when the competitor's side can't be stated from verified sources. */
  reasons: { title: string; body: string; icon: IconKey; competitorLane?: string[]; helpinLane: string[] }[];
  tableLede: string;
  table: { group: string; rows: TableRow[] }[];
  strengths: { title: string; body: string; icon: IconKey }[];
  calculator: Calculator;
  switching: {
    title: string;
    lede: string;
    take: string[];
    setUp: string[];
    steps: { title: string; body: string; status: StepStatus }[];
  };
  faqs: FAQ[];
  closing: { title: string; description: string };
  /** The hero video; its files live in public/new/compare/ (see compareVideo). Without one, the hero shows the glance cards. */
  video?: { seconds: number; summary: string };
  /** Where each competitor fact was checked. Internal record for re-checks; not shown on the page. */
  sources: { label: string; url: string }[];
};

export function formatChecked(date: string) {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });
}

export function competitorSeo(competitor: Competitor): PageSeo {
  return {
    title: `${competitor.seo.title} (${competitor.checked.slice(0, 4)})`,
    description: competitor.seo.description,
    canonicalPath: `/compare/${competitor.slug}`,
    imagePath: `/og/helpin-compare-${competitor.slug}-green-v4.png`,
    imageAlt: `Helpin vs ${competitor.name}`,
  };
}

const VIDEO_PUBLISHED = '2026-09-26';

/** Files and metadata for a competitor's comparison video. Bump the -v suffix when a video is re-cut. */
export function compareVideo(competitor: Competitor) {
  if (!competitor.video) return null;
  const base = `/new/compare/helpin-vs-${competitor.slug}`;
  return {
    ...competitor.video,
    title: `Helpin vs ${competitor.name} in ${competitor.video.seconds} seconds`,
    src: `${base}-1080p-v1.mp4`,
    poster: `${base}-poster-1920-v1.webp`,
    published: VIDEO_PUBLISHED,
  };
}

/** The status a cell shows: explicit when given, otherwise implied by a yes/no value. */
export function cellStatus(value: Cell, status?: Status): Status | undefined {
  if (status) return status;
  if (value === true) return 'yes';
  if (value === false) return 'no';
  return undefined;
}

const CHECKED = '2026-09-26';
/** Plane and Jira were added later, from sources checked on this date. */
const CHECKED_LATER = '2026-09-28';

// Shared Helpin facts, so every page states them the same way.
const HELPIN = {
  seats: 'Unlimited teammates on every plan',
  ai: 'AI usage allowance included in every Cloud plan',
  selfHost: 'Free Community edition (AGPL-3.0, 0.2 beta)',
  trial: '14 days, no card',
  channels: 'Web chat and email',
  sdks: 'Web: JavaScript, React, Next.js, Vue',
  helpCenter: 'Custom domain, AI answers, API reference',
  reporting: 'Knowledge-gap and project reports',
  projects: 'Roadmaps, sprints, objectives, epics',
  crm: 'Contacts, companies, deals',
  meetings: 'Meet, Zoom, Teams and Webex',
  coding: 'Opens GitHub PRs and GitLab MRs for review',
  mcp: 'Hosted and self-hosted (beta)',
};

const row = (label: string, helpin: Cell, competitor: Cell, [helpinStatus, competitorStatus]: [Status?, Status?] = []): TableRow =>
  ({ label, helpin, competitor, helpinStatus, competitorStatus });

const RUN_ALONGSIDE = (name: string) => ({
  title: `Run Helpin alongside ${name}`,
  body: 'Install the widget on a few pages and connect a support address. Move queues over when your team is ready, ideally before your next renewal.',
  status: 'Available now' as const,
});

const REBUILD_DOCS = {
  title: 'Rebuild your help center',
  body: 'Recreate your articles in Helpin Knowledge and publish them on your own domain.',
  status: 'Available now' as const,
};

const SUPPORT_SETUP = [
  'Chat widget on your site or app',
  'Support address, forwarded to Helpin',
  'Help center on your own domain',
  'Team inboxes, tags and saved replies',
  'AI agent guidance and approval rules',
];

const PRICE_LANE = ['Workspace price', 'AI usage included'];
const MODELS_LANE = ['Your AI provider', 'Your keys'];
const DOCS_LANE = ['Conversations', 'Code changes', 'Draft update'];
const LOOP_LANE = ['Ticket', 'Task', 'Pull request', 'Customer told'];
const LOOP_BODY = 'In Helpin, agents can run the whole loop: turn the ticket into a task, have a coding agent open the fix, and tell the customer when it ships. You choose which steps need a person’s approval.';
const MODELS_BODY = 'Helpin’s agents run on the provider and models you choose: always when you self-host, and on Helpin Cloud’s Enterprise plan.';
const NO_LICENSE = 'Self-hosted, there’s no license fee at all.';

/** Questions every comparison answers the same way. */
function sharedFaqs(name: string, { selfHost = true } = {}): FAQ[] {
  return [
    [`Can we run Helpin alongside ${name}?`, `Yes. Add the Helpin widget to a few pages or forward one support address, and keep ${name} for everything else while your team tries the workflow. Move the rest when you’re ready.`],
    ['How is AI billed in Helpin?', 'Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly.'],
    ['Is there a free trial?', 'Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage.'],
    ...(selfHost ? [['Can we self-host Helpin?', 'Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It is a 0.2 beta and includes every product, coding agents too.'] as const] : []),
    ['Will you help us switch?', 'Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over.'],
  ];
}

export const COMPETITORS: Competitor[] = [
  {
    slug: 'intercom',
    name: 'Intercom',
    group: 'Customer support',
    category: 'AI customer support',
    cardLine: 'Per-seat plans plus a fee per AI outcome, compared with one workspace price and connected product work.',
    checked: CHECKED,
    seo: {
      title: 'Open-Source Intercom Alternative: Helpin vs Intercom',
      description: 'Compare Helpin and Intercom: open-source AI support on Cloud or self-hosted, with no per-seat or per-resolution fees, plus projects, CRM and meetings.',
    },
    hero: {
      accent: 'when the answer needs a fix.',
      lede: 'Both answer customers with AI. Intercom is a mature support platform with wide channel coverage. Helpin is open source, runs on Helpin Cloud or your own servers, and keeps the question, the fix and the follow-up on one customer history.',
    },
    glance: [
      { label: 'Pricing', competitor: 'Per seat, plus $0.99 per AI outcome', helpin: 'One workspace price, AI usage included' },
      { label: 'Beyond support', competitor: 'Projects and CRM through integrations', helpin: 'Projects, CRM and meetings built in' },
      { label: 'Hosting', competitor: 'Hosted by Intercom', helpin: 'Open source: Cloud or your servers' },
    ],
    summary: {
      title: 'Different tools for different teams.',
      lede: 'If support is a self-contained function, Intercom is a strong choice. If customer questions regularly turn into product work, Helpin keeps that work connected.',
      competitor: [
        'You need many channels in one inbox: phone, WhatsApp, SMS, social and Slack.',
        'You need native iOS and Android SDKs or a large integration marketplace.',
        'You need SSO, SLA policies and detailed support reporting today.',
      ],
      helpin: [
        'Customer questions often become bugs, features or project tasks.',
        'You want agents to take a request from ticket to fix to customer update, with the approvals you choose.',
        'You want one price per workspace instead of per seat and per AI outcome.',
        'You want an open-source product on Helpin Cloud or your own servers, running on your own AI models.',
        'You want docs that update from support conversations and the code you ship.',
      ],
    },
    reasons: [
      {
        title: 'No per-seat or per-resolution fees',
        icon: 'billing',
        competitorLane: ['Per seat', '+ $0.99 per outcome'],
        helpinLane: PRICE_LANE,
        body: `Intercom charges for every full seat, plus $0.99 for each Fin outcome. Helpin charges one price per workspace with unlimited teammates, and each Cloud plan includes an AI usage allowance, with metered overage only if you turn it on. ${NO_LICENSE}`,
      },
      {
        title: 'Bring your own AI models',
        icon: 'models',
        competitorLane: ['Fin AI Engine', 'Billed per outcome'],
        helpinLane: MODELS_LANE,
        body: `Fin runs on Intercom’s own AI engine and is billed per outcome. ${MODELS_BODY}`,
      },
      {
        title: 'Docs that keep up with the product',
        icon: 'docs',
        competitorLane: ['Conversations', 'Article suggestions'],
        helpinLane: DOCS_LANE,
        body: 'Intercom suggests article updates from support conversations. Helpin does too, and also drafts updates from the code changes you ship, because the docs, the tasks and the repository are connected. Nothing goes live until your team publishes it.',
      },
      {
        title: 'From ticket to fix to customer, on its own',
        icon: 'loop',
        competitorLane: ['Conversation', 'Jira or another tool'],
        helpinLane: LOOP_LANE,
        body: `Intercom resolves the conversation and hands product work to tools like Jira. ${LOOP_BODY}`,
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Pricing',
        rows: [
          row('Price model', 'Per workspace', 'Per seat: $29, $85 or $132 a month billed annually'),
          row('Teammates', HELPIN.seats, 'Each full seat is paid; Lite seats on higher plans'),
          row('AI agent', HELPIN.ai, 'Fin at $0.99 per outcome'),
          row('Free trial', HELPIN.trial, '14 days, no card'),
          row('Open source and self-hosting', HELPIN.selfHost, false, ['yes']),
        ],
      },
      {
        group: 'Support',
        rows: [
          row('Channels', HELPIN.channels, 'Chat, email, phone, WhatsApp, SMS, social, Slack and more', ['partial', 'yes']),
          row('AI answers with human handoff', true, true),
          row('Help center', HELPIN.helpCenter, 'Public help center; multilingual on Advanced', ['yes', 'yes']),
          row('Widget SDKs', HELPIN.sdks, 'Web, iOS, Android, React Native', ['partial', 'yes']),
          row('Support reporting', HELPIN.reporting, 'Pre-built reports; custom reports on Advanced', ['partial', 'yes']),
          row('SSO and SLA policies', false, 'Expert plan', [undefined, 'yes']),
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          row('Project management', HELPIN.projects, 'Through integrations such as Jira', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, 'Contacts and companies; deals through integrations', ['yes', 'partial']),
          row('Meeting notes', HELPIN.meetings, false, ['yes']),
          row('Coding agents', HELPIN.coding, false, ['yes']),
          row('MCP server', HELPIN.mcp, 'Hosted', ['yes', 'yes']),
        ],
      },
    ],
    strengths: [
      { icon: 'channels', title: 'Channels.', body: 'Phone and voice AI, WhatsApp, SMS, social channels, Slack, Discord and Microsoft Teams. Helpin supports web chat and email.' },
      { icon: 'ecosystem', title: 'Ecosystem.', body: 'More than 450 apps and integrations, and native mobile SDKs for iOS and Android.' },
      { icon: 'maturity', title: 'Maturity.', body: 'Over 30,000 customers and years of support-specific features, including SLAs, SSO and custom reports.' },
      { icon: 'messaging', title: 'Proactive messaging.', body: 'Product tours, surveys and targeted in-app messages through add-ons.' },
    ],
    calculator: {
      seatsLabel: 'Support teammates',
      seatsUnit: ['teammate', 'teammates'],
      seats: 8,
      plans: [{ name: 'Essential', annual: 29, monthly: 39 }, { name: 'Advanced', annual: 85, monthly: 99 }, { name: 'Expert', annual: 132 }],
      plan: 1,
      helpinPlan: 'growth',
      ai: { label: 'Fin AI outcomes a month', price: 0.99, volume: 1000, max: 5000, step: 100 },
      notes: ['Intercom’s channel usage (phone, SMS, WhatsApp) and add-ons such as Copilot and Proactive Support are not included.'],
    },
    switching: {
      title: 'Moving from Intercom.',
      lede: 'Start with part of your support, then move the rest when you’re ready.',
      take: ['Conversations, as CSV or through the API', 'Contacts and companies', 'Help center articles', 'Saved replies and tags you want to keep'],
      setUp: SUPPORT_SETUP,
      steps: [
        RUN_ALONGSIDE('Intercom'),
        REBUILD_DOCS,
        { title: 'Import conversations and contacts', status: 'Coming soon', body: 'An Intercom importer is coming soon. Until then, Intercom exports conversations as CSV or through its API.' },
      ],
    },
    faqs: [
      ['Is Helpin a good Intercom alternative?', 'It is if your team wants AI support connected to product work, CRM and meetings, or wants to self-host. If you depend on phone, WhatsApp or social channels, or on native mobile SDKs, Intercom covers more today.'],
      ['Is Helpin cheaper than Intercom?', 'For most teams with several support teammates, yes. Helpin charges one price per workspace with AI usage included. Intercom charges per seat plus $0.99 per Fin outcome. Use the calculator above with your own team size and volume.'],
      ['Is Intercom now called Fin?', 'The company renamed itself Fin in May 2026 and was acquired by Salesforce in September 2026. The helpdesk product is still called Intercom.'],
      ['Can Helpin import my Intercom data?', 'An Intercom importer is coming soon. You can export conversations from Intercom today and start Helpin alongside it while you move over.'],
      ...sharedFaqs('Intercom'),
    ],
    closing: { title: 'Bring support and the work behind it together.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    video: { seconds: 39, summary: 'Intercom charges per seat plus $0.99 per AI outcome. Helpin charges one workspace price with AI usage included, and turns the answer into a task, a pull request and a follow-up.' },
    sources: [
      { label: 'Intercom plans explained', url: 'https://www.intercom.com/help/en/articles/9061614-fin-and-intercom-plans-explained' },
      { label: 'Intercom seats', url: 'https://www.intercom.com/help/en/articles/8205716-seats' },
      { label: 'Fin AI agent outcomes', url: 'https://www.intercom.com/help/en/articles/8205718-fin-ai-agent-outcomes' },
      { label: 'Intercom pricing FAQ', url: 'https://www.intercom.com/help/en/articles/8344190-pricing-faqs' },
      { label: 'Intercom free trial', url: 'https://www.intercom.com/help/en/articles/891-how-to-start-a-free-trial-of-fin-and-intercom' },
      { label: 'Intercom mobile SDK FAQ', url: 'https://www.intercom.com/help/en/articles/7669340-mobile-sdk-faqs' },
      { label: 'Intercom MCP server', url: 'https://developers.intercom.com/docs/guides/mcp' },
      { label: 'Intercom conversation export', url: 'https://www.intercom.com/help/en/articles/2046229-export-your-conversations-data' },
      { label: 'Salesforce completes acquisition of Fin', url: 'https://investor.salesforce.com/news/news-details/2026/Salesforce-Completes-Acquisition-of-Fin/default.aspx' },
    ],
  },
  {
    slug: 'zendesk',
    name: 'Zendesk',
    group: 'Customer support',
    category: 'Help desk',
    cardLine: 'An established omnichannel help desk, compared with support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Open-Source Zendesk Alternative: Helpin vs Zendesk',
      description: 'Compare Helpin and Zendesk: an open-source help desk on Cloud or your servers, your own AI models, no per-agent fees, and support tied to projects.',
    },
    hero: {
      accent: 'one history from ticket to release.',
      lede: 'Zendesk is an established help desk with deep omnichannel and contact center features. Helpin is a newer, open-source platform that keeps support, projects, CRM, meetings and docs on one customer history.',
    },
    glance: [
      { label: 'Pricing', competitor: 'Per agent, plus AI per resolution', helpin: 'One workspace price, AI usage included' },
      { label: 'CRM', competitor: 'Zendesk Sell retires in 2027', helpin: 'Contacts, companies and deals built in' },
      { label: 'Hosting', competitor: 'Hosted by Zendesk', helpin: 'Open source: Cloud or your servers' },
    ],
    summary: {
      title: 'A help desk, or a connected workspace.',
      lede: 'Zendesk is built for large, multichannel service teams. Helpin suits product-led teams whose support questions turn into product and account work.',
      competitor: [
        'You run a large service operation with voice, IVR and many channels.',
        'You need enterprise features such as SSO, SLAs, sandboxes and approval workflows.',
        'You rely on a large marketplace of more than 1,800 apps.',
      ],
      helpin: [
        'Support questions regularly become roadmap and engineering work.',
        'You want CRM, meeting notes and projects in the same product as support.',
        'You want one price per workspace, with unlimited teammates and no per-resolution fees.',
        'You want an open-source product on Helpin Cloud or your own servers, running on your own AI models.',
        'You want docs that update from support conversations and the code you ship.',
      ],
    },
    reasons: [
      {
        title: 'No per-agent or per-resolution fees',
        icon: 'billing',
        competitorLane: ['Per agent', '+ AI resolutions'],
        helpinLane: PRICE_LANE,
        body: `Zendesk charges per agent, and AI agent resolutions beyond the included allowance are billed on top, as is Copilot at $50 per agent. Helpin charges one price per workspace with unlimited teammates, and each Cloud plan includes an AI usage allowance. ${NO_LICENSE}`,
      },
      {
        title: 'Bring your own AI models',
        icon: 'models',
        competitorLane: ['Zendesk-managed AI', 'Own key through apps'],
        helpinLane: MODELS_LANE,
        body: `Zendesk’s AI agents run on models Zendesk manages; using your own OpenAI key goes through Marketplace apps or the API. ${MODELS_BODY}`,
      },
      {
        title: 'Docs that keep up with the product',
        icon: 'docs',
        competitorLane: ['Tickets', 'Article drafts'],
        helpinLane: DOCS_LANE,
        body: 'Zendesk’s Knowledge Builder drafts help center articles from ticket data. Helpin drafts updates from support conversations and from the code changes you ship, so the docs change when the product does. Nothing goes live until your team publishes it.',
      },
      {
        title: 'From ticket to fix to customer, on its own',
        icon: 'loop',
        competitorLane: ['Ticket', 'Jira or another tool'],
        helpinLane: LOOP_LANE,
        body: `Zendesk manages the ticket and connects to tools like Jira for engineering work. ${LOOP_BODY}`,
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Pricing',
        rows: [
          row('Price model', 'Per workspace', 'Per agent: Suite Team $55, Professional $115 a month billed annually'),
          row('Teammates', HELPIN.seats, 'Each agent is paid'),
          row('AI agent', HELPIN.ai, 'Billed per verified resolution, beyond a small included allowance'),
          row('Free trial', HELPIN.trial, '14 days, no card'),
          row('Open source and self-hosting', HELPIN.selfHost, false, ['yes']),
        ],
      },
      {
        group: 'Support',
        rows: [
          row('Channels', HELPIN.channels, 'Email, messaging, chat, voice with IVR, social', ['partial', 'yes']),
          row('AI answers with human handoff', true, true),
          row('Help center', HELPIN.helpCenter, 'Knowledge base on Suite plans', ['yes', 'yes']),
          row('Widget SDKs', HELPIN.sdks, 'Web, iOS, Android', ['partial', 'yes']),
          row('Support reporting', HELPIN.reporting, 'Dashboards and analytics', ['partial', 'yes']),
          row('SSO and SLA policies', false, true),
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          row('Project management', HELPIN.projects, 'Through integrations such as Jira', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, 'Zendesk Sell, retiring in August 2027', ['yes', 'partial']),
          row('Meeting notes', HELPIN.meetings, false, ['yes']),
          row('Coding agents', HELPIN.coding, false, ['yes']),
          row('MCP server', HELPIN.mcp, 'Announced; MCP client in early access', ['yes', 'partial']),
        ],
      },
    ],
    strengths: [
      { icon: 'channels', title: 'Omnichannel and voice.', body: 'Native contact center, IVR and messaging channels. Helpin supports web chat and email.' },
      { icon: 'enterprise', title: 'Enterprise controls.', body: 'SSO, SLA policies, sandboxes, custom roles and approval workflows.' },
      { icon: 'ecosystem', title: 'Ecosystem.', body: 'More than 1,800 marketplace apps and mobile SDKs for iOS and Android.' },
      { icon: 'reporting', title: 'Reporting.', body: 'Mature dashboards and analytics for large service teams.' },
    ],
    calculator: {
      seatsLabel: 'Support agents',
      seatsUnit: ['agent', 'agents'],
      seats: 8,
      plans: [{ name: 'Suite Team', annual: 55 }, { name: 'Suite Professional', annual: 115 }],
      plan: 0,
      helpinPlan: 'growth',
      notes: [
        'Zendesk bills AI agent resolutions beyond a small included allowance, and doesn’t publish the price, so they aren’t included.',
        'Copilot is an extra $50 per agent. Zendesk shows annual prices only.',
      ],
    },
    switching: {
      title: 'Moving from Zendesk.',
      lede: 'Start with part of your support, then move the rest when you’re ready.',
      take: ['Tickets and users (ask Zendesk to enable exports)', 'Organizations', 'Help center articles', 'Macros you want to keep as saved replies'],
      setUp: SUPPORT_SETUP,
      steps: [
        RUN_ALONGSIDE('Zendesk'),
        REBUILD_DOCS,
        { title: 'Import tickets and contacts', status: 'Coming soon', body: 'A Zendesk importer is coming soon. Until then, Zendesk account owners can request ticket and user exports.' },
      ],
    },
    faqs: [
      ['Is Helpin a good Zendesk alternative?', 'It is for product-led teams that want support connected to projects, CRM and meetings, or that want to self-host. Large contact centers that need voice, IVR and many channels are better served by Zendesk today.'],
      ['Is Helpin cheaper than Zendesk?', 'For most teams with several agents, yes. Helpin charges one price per workspace with an AI allowance included. Zendesk charges per agent, with AI resolutions and Copilot billed separately. Try the calculator above with your own numbers.'],
      ['What happens to Zendesk Sell?', 'Zendesk has announced that Zendesk Sell will be retired on August 31, 2027. Helpin includes a CRM with contacts, companies and deals.'],
      ['Can Helpin import my Zendesk data?', 'A Zendesk importer is coming soon. You can export data from Zendesk today and start Helpin alongside it while you move over.'],
      ...sharedFaqs('Zendesk'),
    ],
    closing: { title: 'Take the ticket all the way to the release.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    video: { seconds: 39, summary: 'The ticket says solved while the customer is still stuck. Helpin keeps one history from ticket to release, with CRM built in and one price for unlimited teammates.' },
    sources: [
      { label: 'Zendesk pricing', url: 'https://www.zendesk.com/pricing/' },
      { label: 'Zendesk AI agent resolutions', url: 'https://support.zendesk.com/hc/en-us/articles/9570369117338' },
      { label: 'Zendesk AI allowance', url: 'https://support.zendesk.com/hc/en-us/articles/10479587390106' },
      { label: 'Zendesk Sell retirement', url: 'https://support.zendesk.com/hc/en-us/articles/9591462550042' },
      { label: 'Zendesk data export', url: 'https://support.zendesk.com/hc/en-us/articles/4408886165402' },
      { label: 'Zendesk MCP client', url: 'https://support.zendesk.com/hc/en-us/articles/10889847373466' },
      { label: 'Zendesk Marketplace', url: 'https://www.zendesk.com/blog/zendesk-marketplace/' },
    ],
  },
  {
    slug: 'help-scout',
    name: 'Help Scout',
    group: 'Customer support',
    category: 'Shared inbox',
    cardLine: 'A simple shared inbox priced per user, compared with support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Help Scout: Open-Source Alternative',
      description: 'Compare Helpin and Help Scout: open-source support on Cloud or self-hosted, AI without per-resolution fees, and docs that update from tickets and code.',
    },
    hero: {
      accent: 'a simple inbox, and what comes after it.',
      lede: 'Help Scout is a well-loved shared inbox that keeps support simple. Helpin is open source, on Helpin Cloud or your own servers, and adds projects, CRM, meetings and AI agents on the same customer history.',
    },
    glance: [
      { label: 'Pricing', competitor: 'Per user, plus $0.75 per AI resolution', helpin: 'One workspace price, AI usage included' },
      { label: 'Beyond support', competitor: 'Projects and CRM through integrations', helpin: 'Projects, CRM and meetings built in' },
      { label: 'Hosting', competitor: 'Hosted by Help Scout', helpin: 'Open source: Cloud or your servers' },
    ],
    summary: {
      title: 'Simple support, or support connected to everything else.',
      lede: 'Both are friendly for small teams. The difference is how far past the inbox you want to go.',
      competitor: [
        'You want an email-like inbox with a free plan for up to five users.',
        'You need WhatsApp, Instagram, Messenger or SMS in the inbox.',
        'You want a built-in importer to bring over conversations from another help desk.',
      ],
      helpin: [
        'Support questions turn into product work you want to track and ship.',
        'You want CRM, meeting notes and projects alongside support.',
        'You want one workspace price with AI usage included, instead of per-user and per-resolution fees.',
        'You want an open-source product on Helpin Cloud or your own servers, running on your own AI models.',
        'You want docs that update from support conversations and the code you ship.',
      ],
    },
    reasons: [
      {
        title: 'No per-user or per-resolution fees',
        icon: 'billing',
        competitorLane: ['Per user', '+ $0.75 per resolution'],
        helpinLane: PRICE_LANE,
        body: `Help Scout charges per user, plus $0.75 for each AI resolution. Helpin charges one price per workspace with unlimited teammates, and each Cloud plan includes an AI usage allowance. ${NO_LICENSE}`,
      },
      {
        title: 'Bring your own AI models',
        icon: 'models',
        competitorLane: ['AI Answers', 'Billed per resolution'],
        helpinLane: MODELS_LANE,
        body: `Help Scout’s AI Answers is billed per resolution. Helpin has no per-resolution fees. ${MODELS_BODY}`,
      },
      {
        title: 'Docs that keep up with the product',
        icon: 'docs',
        helpinLane: [...DOCS_LANE, 'Published'],
        body: 'Helpin’s agents draft help center updates from unanswered questions and from the code changes you ship, because the docs, the tasks and the repository live in one product. Nothing goes live until your team publishes it.',
      },
      {
        title: 'From ticket to fix to customer, on its own',
        icon: 'loop',
        competitorLane: ['Conversation', 'Integrations'],
        helpinLane: LOOP_LANE,
        body: `Help Scout focuses on conversations and docs, and connects to other tools for product work. ${LOOP_BODY}`,
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Pricing',
        rows: [
          row('Price model', 'Per workspace', 'Per user: Standard $25, Plus $45 a month billed annually'),
          row('Free plan', 'Self-hosted Community edition', 'Up to 5 users and 100 contacts a month'),
          row('AI agent', HELPIN.ai, 'AI Answers at $0.75 per resolution'),
          row('Free trial', HELPIN.trial, '15 days, no card'),
          row('Open source and self-hosting', HELPIN.selfHost, false, ['yes']),
        ],
      },
      {
        group: 'Support',
        rows: [
          row('Channels', HELPIN.channels, 'Email, chat, WhatsApp, Instagram, Messenger, SMS', ['partial', 'yes']),
          row('AI answers with human handoff', true, true),
          row('Help center', HELPIN.helpCenter, 'Docs sites; 2 on Standard, 3 on Plus', ['yes', 'yes']),
          row('Widget SDKs', HELPIN.sdks, 'Web, iOS, Android', ['partial', 'yes']),
          row('Support reporting', HELPIN.reporting, 'Reports; history length depends on plan', ['partial', 'yes']),
          row('SLA policies', false, 'Limited on Standard and Plus', [undefined, 'partial']),
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          row('Project management', HELPIN.projects, 'Through the Jira integration on Plus', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, 'Customer properties and company profiles', ['yes', 'partial']),
          row('Meeting notes', HELPIN.meetings, false, ['yes']),
          row('Coding agents', HELPIN.coding, false, ['yes']),
          row('MCP server', HELPIN.mcp, 'Hosted, read-only', ['yes', 'partial']),
        ],
      },
    ],
    strengths: [
      { icon: 'simplicity', title: 'Simplicity.', body: 'A calm, email-like inbox that reviewers consistently praise for ease of use.' },
      { icon: 'channels', title: 'Channels.', body: 'WhatsApp, Instagram, Messenger and SMS on paid plans. Helpin supports web chat and email.' },
      { icon: 'import', title: 'Free plan and migration.', body: 'A free plan for up to five users, and a built-in importer for conversations from Zendesk, Intercom and more than 30 other tools.' },
      { icon: 'mobile', title: 'Mobile.', body: 'Beacon SDKs for iOS and Android.' },
    ],
    calculator: {
      seatsLabel: 'Support teammates',
      seatsUnit: ['teammate', 'teammates'],
      seats: 8,
      plans: [{ name: 'Standard', annual: 25 }, { name: 'Plus', annual: 45 }, { name: 'Pro', annual: 75, minSeats: 10 }],
      plan: 1,
      helpinPlan: 'growth',
      ai: { label: 'AI resolutions a month', price: 0.75, volume: 300, max: 3000, step: 50 },
      notes: [
        'Help Scout’s monthly-billing prices aren’t shown on its pricing page, so annual rates are used. Prepaying for AI resolutions can lower the rate.',
        'For one or two users, Help Scout’s free and Standard plans can cost less than Helpin.',
      ],
    },
    switching: {
      title: 'Moving from Help Scout.',
      lede: 'Bring your docs over today, and keep older conversations where they are.',
      take: ['Docs articles, imported directly into Helpin', 'Your customer list', 'Saved replies and tags you want to keep'],
      setUp: SUPPORT_SETUP,
      steps: [
        RUN_ALONGSIDE('Help Scout'),
        { title: 'Import your Help Scout Docs', status: 'Available now', body: 'Bring your Help Scout Docs articles into Helpin Knowledge.' },
        { title: 'Move conversation history', status: 'Not yet', body: 'Conversation history isn’t imported yet. Keep Help Scout available for older threads while new ones arrive in Helpin.' },
      ],
    },
    faqs: [
      ['Is Helpin a good Help Scout alternative?', 'It is if you want support connected to projects, CRM and meetings, AI usage included in the price, or the option to self-host. If you mainly need a simple inbox with social channels, Help Scout covers that well.'],
      ['Does Help Scout charge per contact?', 'Help Scout moved to contact-based billing in 2024 and later returned to per-user pricing for new customers. Some existing accounts still use contact-based billing.'],
      ['Can Helpin import from Help Scout?', 'Yes, for Help Scout Docs articles. Conversation history is not imported yet.'],
      ['Is Helpin cheaper than Help Scout?', 'It depends on team size. Help Scout can cost less for one or two users, especially on its free plan. With more teammates or regular AI resolutions, Helpin’s single workspace price usually costs less. The calculator above shows both.'],
      ...sharedFaqs('Help Scout'),
    ],
    closing: { title: 'Keep support simple, and connect what comes next.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    video: { seconds: 39, summary: 'Keep a calm inbox and connect what comes next: projects, CRM, meetings and AI agents on one customer history, with AI usage included in one workspace price.' },
    sources: [
      { label: 'Help Scout pricing', url: 'https://www.helpscout.com/pricing/' },
      { label: 'Help Scout AI agent', url: 'https://www.helpscout.com/agent/' },
      { label: 'Help Scout contact-based billing', url: 'https://docs.helpscout.com/article/1593-contact-based-billing-and-plans-guide' },
      { label: 'Help Scout price and plans guide', url: 'https://docs.helpscout.com/article/596-price-and-plans-guide' },
      { label: 'Help Scout imports', url: 'https://docs.helpscout.com/article/1495-import-emails-or-tickets-into-help-scout' },
      { label: 'Help Scout MCP server', url: 'https://articles.helpscout.com/blog/introducing-help-scout-mcp/' },
    ],
  },
  {
    slug: 'chatwoot',
    name: 'Chatwoot',
    group: 'Customer support',
    category: 'Open-source support',
    cardLine: 'Two open-source options: a multichannel inbox, or support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Open-Source Chatwoot Alternative: Helpin vs Chatwoot',
      description: 'Compare Helpin and Chatwoot, two open-source customer support platforms: channels, AI, self-hosting costs, and projects, CRM and meetings.',
    },
    hero: {
      accent: 'two open-source ways to run support.',
      lede: 'Both are open source and both can run on your own servers. Chatwoot is a mature, multichannel inbox. Helpin is a newer platform that connects support to projects, CRM, meetings and docs, with AI agents working across them.',
    },
    glance: [
      { label: 'License', competitor: 'MIT core, paid enterprise edition', helpin: 'AGPL-3.0 for every product feature' },
      { label: 'Self-hosted AI', competitor: 'Captain needs a paid plan', helpin: 'AI agents included' },
      { label: 'Beyond support', competitor: 'Issues through the Linear integration', helpin: 'Projects, CRM and meetings built in' },
    ],
    summary: {
      title: 'Both open. Built for different jobs.',
      lede: 'Chatwoot focuses on channels and the inbox. Helpin focuses on what happens after the conversation.',
      competitor: [
        'You need many channels: WhatsApp, Instagram, Telegram, LINE, TikTok, SMS or voice.',
        'You want a mature project with years of releases and a large community.',
        'You need native mobile apps and SDKs today.',
      ],
      helpin: [
        'Support questions turn into project work you want to plan and ship.',
        'You want CRM with deals and meeting notes alongside support.',
        'You want every product feature open source, with no paid self-hosted tier.',
        'You want AI agents, coding agents included, free when self-hosted on your own AI models.',
        'You want docs that update from support conversations and the code you ship.',
      ],
    },
    reasons: [
      {
        title: 'No per-agent fees or AI credits',
        icon: 'billing',
        competitorLane: ['Per agent', '+ AI credits'],
        helpinLane: PRICE_LANE,
        body: 'Chatwoot Cloud charges per agent, and Captain AI runs on monthly credits, with more at $20 per 1,000. Helpin Cloud charges one price per workspace with unlimited teammates and an AI usage allowance included. Self-hosted, Helpin has no paid tier at all.',
      },
      {
        title: 'Your own AI models, free',
        icon: 'models',
        competitorLane: ['Your key', 'Paid plan needed'],
        helpinLane: ['Your key', 'Free Community edition'],
        body: 'Self-hosted, both run on your own AI provider. Chatwoot’s Captain needs the paid Premium plan or higher for that. Helpin’s agents, coding agents included, are part of the free Community edition, and every Helpin product feature is open source under AGPL-3.0.',
      },
      {
        title: 'Docs that keep up with the product',
        icon: 'docs',
        helpinLane: [...DOCS_LANE, 'Published'],
        body: 'Helpin’s agents draft help center updates from unanswered questions and from the code changes you ship, because the docs, the tasks and the repository live in one product. Nothing goes live until your team publishes it.',
      },
      {
        title: 'From ticket to fix to customer, on its own',
        icon: 'loop',
        competitorLane: ['Conversation', 'Linear issue'],
        helpinLane: LOOP_LANE,
        body: `Chatwoot links conversations to Linear issues. ${LOOP_BODY}`,
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Open source and pricing',
        rows: [
          row('License', 'AGPL-3.0 for every product feature', 'MIT core, with a separate paid enterprise edition', ['yes', 'partial']),
          row('Self-hosted AI agent', 'Included; connect your own AI provider', 'Captain needs a paid plan from $19 an agent', ['yes', 'partial']),
          row('Cloud price model', 'Per workspace', 'Per agent: $19, $39 or $99 a month'),
          row('Cloud AI', HELPIN.ai, 'Captain credits, then $20 per 1,000'),
          row('Free trial', HELPIN.trial, '15 days'),
        ],
      },
      {
        group: 'Support',
        rows: [
          row('Channels', HELPIN.channels, 'Chat, email, WhatsApp, social, Telegram, LINE, SMS, voice', ['partial', 'yes']),
          row('Help center', HELPIN.helpCenter, 'Built in, with search and locales', ['yes', 'yes']),
          row('Widget SDKs', HELPIN.sdks, 'Web, React Native, Flutter', ['partial', 'yes']),
          row('Support reporting', HELPIN.reporting, 'Agent, team, inbox and CSAT reports', ['partial', 'yes']),
          row('SLA policies and SSO', false, 'Enterprise plan', [undefined, 'partial']),
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          row('Project management', HELPIN.projects, 'Through the Linear integration', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, 'Contacts, companies and segments', ['yes', 'partial']),
          row('Meeting notes', HELPIN.meetings, false, ['yes']),
          row('Coding agents', HELPIN.coding, false, ['yes']),
          row('MCP server', HELPIN.mcp, 'Community-built only', ['yes', 'partial']),
        ],
      },
    ],
    strengths: [
      { icon: 'channels', title: 'Channels.', body: 'WhatsApp, Facebook, Instagram, TikTok, Telegram, LINE, SMS and voice. Helpin supports web chat and email.' },
      { icon: 'community', title: 'Maturity and community.', body: 'Years of releases, tens of thousands of GitHub stars and roughly monthly releases. Helpin’s Community edition is a 0.2 beta.' },
      { icon: 'deploy', title: 'Deployment options.', body: 'Official Docker, Kubernetes Helm charts, a Linux installer and cloud marketplace images.' },
      { icon: 'mobile', title: 'Mobile.', body: 'An agent app for iOS and Android, and mobile widget SDKs.' },
    ],
    calculator: {
      seatsLabel: 'Support agents',
      seatsUnit: ['agent', 'agents'],
      seats: 8,
      plans: [{ name: 'Startups', annual: 19, monthly: 19 }, { name: 'Business', annual: 39, monthly: 39 }, { name: 'Enterprise', annual: 99, monthly: 99 }],
      plan: 1,
      helpinPlan: 'growth',
      notes: [
        'Chatwoot’s Captain AI uses credits: 300 to 800 are included each month depending on the plan, then $20 per 1,000. Credit use per conversation isn’t published, so AI isn’t included here.',
      ],
      selfHosted: {
        plans: [{ name: 'Community', annual: 0, monthly: 0 }, { name: 'Premium', annual: 19, monthly: 19 }, { name: 'Enterprise', annual: 99, monthly: 99 }],
        plan: 1,
        note: 'Self-hosted, both use your own AI provider and exclude your hosting costs. Chatwoot’s Captain AI needs Premium or higher; Helpin’s AI agents are part of the free Community edition.',
      },
    },
    switching: {
      title: 'Moving from Chatwoot.',
      lede: 'Start with part of your support, then move the rest when you’re ready.',
      take: ['Contacts and conversations, through the Chatwoot API', 'Help center articles', 'Saved replies you want to keep'],
      setUp: SUPPORT_SETUP,
      steps: [
        RUN_ALONGSIDE('Chatwoot'),
        REBUILD_DOCS,
        { title: 'Import conversations', status: 'Not yet', body: 'There is no Chatwoot importer yet. Both products have APIs if you need to move records yourself.' },
      ],
    },
    faqs: [
      ['Are Helpin and Chatwoot both open source?', 'Yes. Chatwoot’s core is MIT-licensed with a separate paid enterprise edition. Helpin’s product features are open source under AGPL-3.0.'],
      ['Which is better for self-hosting?', 'Chatwoot is more mature, with more deployment options. Helpin includes AI agents, projects and CRM in the free Community edition, which is currently a 0.2 beta.'],
      ['Can I use my own AI provider?', 'Yes, with both. Self-hosted Helpin connects to OpenAI, Anthropic, OpenRouter or an OpenAI-compatible endpoint. Chatwoot’s Captain supports your own OpenAI-compatible key on a paid plan.'],
      ['Does Helpin support WhatsApp?', 'Not today. Helpin supports web chat and email. If WhatsApp or social channels are essential, Chatwoot covers them.'],
      ...sharedFaqs('Chatwoot', { selfHost: false }),
    ],
    closing: { title: 'Open source, from the conversation to the release.', description: 'Self-host the Community edition for free, or start a 14-day trial of Helpin Cloud with no card.' },
    video: { seconds: 37, summary: 'Both are open source. Chatwoot’s AI agent needs a paid plan even when self-hosted; Helpin’s free Community edition includes AI agents, projects and CRM.' },
    sources: [
      { label: 'Chatwoot pricing', url: 'https://www.chatwoot.com/pricing' },
      { label: 'Chatwoot self-hosted plans', url: 'https://www.chatwoot.com/pricing/self-hosted-plans' },
      { label: 'Chatwoot license', url: 'https://github.com/chatwoot/chatwoot/blob/develop/LICENSE' },
      { label: 'Chatwoot enterprise edition', url: 'https://developers.chatwoot.com/self-hosted/enterprise-edition' },
      { label: 'Captain on self-hosted installs', url: 'https://www.chatwoot.com/hc/user-guide/articles/1755284287-how-to-enable-captain-on-self_hosted-installations' },
      { label: 'Chatwoot features', url: 'https://www.chatwoot.com/features' },
      { label: 'Chatwoot on GitHub', url: 'https://github.com/chatwoot/chatwoot' },
    ],
  },
  {
    slug: 'linear',
    name: 'Linear',
    group: 'Project management',
    category: 'Project management',
    cardLine: 'A fast issue tracker that connects to your support tool, compared with projects and support in one product.',
    checked: CHECKED,
    seo: {
      title: 'Open-Source Linear Alternative: Helpin vs Linear',
      description: 'Compare Helpin Projects and Linear: roadmaps, sprints, coding agents and pricing, and what changes when support, CRM and meetings live alongside the work.',
    },
    hero: {
      accent: 'plan the work with the customer attached.',
      lede: 'Linear is a fast, focused issue tracker that connects to your support tool. Helpin includes projects and support in one product, along with CRM, meetings and docs, so the request, the task and the follow-up share one history.',
    },
    glance: [
      { label: 'Customer requests', competitor: 'Linked from Intercom or Zendesk', helpin: 'Support inbox and CRM built in' },
      { label: 'Pricing', competitor: 'Per user', helpin: 'One workspace price, unlimited teammates' },
      { label: 'Hosting', competitor: 'Hosted by Linear', helpin: 'Open source: Cloud or your servers' },
    ],
    summary: {
      title: 'A focused tracker, or projects with the customer built in.',
      lede: 'Many teams love Linear for engineering. Helpin is for teams that want customer requests and the work behind them in the same place.',
      competitor: [
        'You want a fast, keyboard-first issue tracker for engineering.',
        'You already have a support tool you like and just want to link requests to issues.',
        'You want a wide choice of third-party coding agents and native mobile apps.',
      ],
      helpin: [
        'You want support, projects, CRM and meetings in one product.',
        'You want agents to plan and code from the task and the customer conversation behind it.',
        'You want one price per workspace instead of per user.',
        'You want to self-host an open-source product.',
      ],
    },
    reasons: [
      {
        title: 'Customer requests arrive from another tool',
        icon: 'requests',
        competitorLane: ['Intercom or Zendesk', 'Linear issue'],
        helpinLane: ['Support inbox', 'Task'],
        body: 'Linear links requests from tools like Intercom and Zendesk, which needs its Business plan. In Helpin the support inbox, help center and CRM are part of the product, so the conversation is already attached to the task.',
      },
      {
        title: 'The customer context lives elsewhere',
        icon: 'agents',
        competitorLane: ['Issue', 'Coding agent'],
        helpinLane: ['Conversation and history', 'Task', 'Pull request'],
        body: 'Both can hand an issue to a coding agent. Helpin’s agents also see the customer conversation, earlier workarounds and account context behind the task, and open a pull request for your team to review.',
      },
      {
        title: 'Following up means switching tools',
        icon: 'loop',
        competitorLane: ['Release', 'Support tool', 'Customer'],
        helpinLane: ['Release', 'Same conversation'],
        body: 'When work ships in Linear, the update goes back through your support tool. In Helpin, the same history holds the release and the original conversation, so the team can follow up with the customer from one place.',
      },
      {
        title: 'Per-user pricing, hosted only',
        icon: 'hosting',
        competitorLane: ['Per user', 'Hosted only'],
        helpinLane: ['Per workspace', 'Cloud or self-hosted'],
        body: 'Linear charges per user and is hosted only. Helpin charges per workspace with unlimited teammates, and is open source, so you can self-host it.',
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Pricing',
        rows: [
          row('Price model', 'Per workspace', 'Per user: Basic $10, Business $16 a month billed annually'),
          row('Free plan', 'Self-hosted Community edition', 'Unlimited members, 2 teams, 250 issues'),
          row('AI coding', 'From the AI allowance included in every Cloud plan', 'Prepaid AI credits for coding sessions'),
          row('Open source and self-hosting', HELPIN.selfHost, false, ['yes']),
        ],
      },
      {
        group: 'Planning',
        rows: [
          row('Roadmaps and sprints', true, 'Initiatives, projects and cycles', [undefined, 'yes']),
          row('Triage', true, true),
          row('GitHub and GitLab', 'PR and MR linking', 'PR and MR linking', ['yes', 'yes']),
          row('Coding agents', HELPIN.coding, 'Linear Agent plus Cursor, Codex, Copilot and others', ['yes', 'yes']),
          row('Delivery reporting', 'Velocity and sprint reports', 'Insights and dashboards on Business', ['yes', 'yes']),
          row('Native mobile apps', false, 'iOS and Android', [undefined, 'yes']),
        ],
      },
      {
        group: 'Customers',
        rows: [
          row('Support inbox and live chat', HELPIN.channels, 'Integrations with Intercom and Zendesk on Business', ['yes', 'partial']),
          row('Help center', HELPIN.helpCenter, false, ['yes']),
          row('CRM with deals', HELPIN.crm, false, ['yes']),
          row('Meeting notes', HELPIN.meetings, 'Gong transcripts on Enterprise', ['yes', 'partial']),
          row('MCP server', HELPIN.mcp, 'Hosted', ['yes', 'yes']),
        ],
      },
    ],
    strengths: [
      { icon: 'speed', title: 'Speed and design.', body: 'A fast, keyboard-first tracker that engineering teams consistently praise.' },
      { icon: 'community', title: 'Adoption.', body: 'Used by more than 40,000 companies, with a large community of templates and practices.' },
      { icon: 'agents', title: 'Agent ecosystem.', body: 'Assign issues to Linear Agent or to third-party agents such as Cursor, Codex, Copilot and Devin.' },
      { icon: 'mobile', title: 'Mobile and imports.', body: 'Native iOS and Android apps, and importers for Jira, GitHub Issues, Asana and Shortcut.' },
    ],
    calculator: {
      seatsLabel: 'People on the team',
      seatsUnit: ['person', 'people'],
      seats: 20,
      plans: [{ name: 'Basic', annual: 10 }, { name: 'Business', annual: 16 }],
      plan: 1,
      helpinPlan: 'growth',
      notes: [
        'Linear’s coding sessions use prepaid AI credits, and Linear shows annual prices only.',
        'Linear needs a separate support tool; Business includes the Intercom and Zendesk integrations. Helpin’s price includes support, CRM and meetings.',
      ],
    },
    switching: {
      title: 'Moving from Linear.',
      lede: 'You don’t have to move everything at once. Many teams start with support.',
      take: ['Issues, as CSV or through the API', 'Projects and cycles you want to plan in Helpin', 'Specs and docs'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Linear connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Linear, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Linear’s tools through MCP, so engineering can stay in Linear while support runs in Helpin.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Linear issues', status: 'Not yet', body: 'There is no Linear importer yet. Linear exports issues to CSV and through its API. Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      ['Is Helpin a Linear alternative?', 'For teams that want projects and customer support in one product, yes. If you want a dedicated issue tracker and already use a separate support tool, Linear is a strong choice.'],
      ['Can I use Helpin and Linear together?', 'Yes. Helpin’s agents can use Linear’s tools through MCP, so support can run in Helpin while engineering stays in Linear.'],
      ['Does Linear have customer support features?', 'Linear has no customer-facing inbox, live chat or help center. It links customer requests from support tools such as Intercom and Zendesk.'],
      ['Do both have coding agents?', 'Yes. Linear Agent can write code in cloud sessions, and third-party agents can be assigned issues. Helpin’s coding agents work from the task and the customer conversation behind it, and open pull requests for review.'],
      ['Can Helpin import from Linear?', 'Not yet. Linear exports issues to CSV and through its API, and Helpin imports projects from Shortcut today.'],
      ...sharedFaqs('Linear').filter(([question]) => !question.startsWith('Can we run Helpin alongside')),
    ],
    closing: { title: 'Plan the work with the customer in view.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    video: { seconds: 35, summary: 'Linear links customer requests from your support tool. Helpin keeps the customer attached to the task, from the support inbox to the pull request and the follow-up.' },
    sources: [
      { label: 'Linear pricing', url: 'https://linear.app/pricing' },
      { label: 'Linear customer requests', url: 'https://linear.app/docs/customer-requests' },
      { label: 'Linear AI credits', url: 'https://linear.app/docs/ai-credits' },
      { label: 'Linear coding sessions', url: 'https://linear.app/changelog/2026-06-11-coding-sessions' },
      { label: 'Agents in Linear', url: 'https://linear.app/docs/agents-in-linear' },
      { label: 'Linear imports', url: 'https://linear.app/docs/import-issues' },
      { label: 'Linear exports', url: 'https://linear.app/docs/exporting-data' },
      { label: 'Linear MCP server', url: 'https://linear.app/docs/mcp' },
      { label: 'Linear mobile apps', url: 'https://linear.app/mobile' },
    ],
  },
  {
    slug: 'plane',
    name: 'Plane',
    group: 'Project management',
    category: 'Open-source project management',
    cardLine: 'Two open-source options: a project tracker, or projects with support, CRM and meetings on one customer history.',
    checked: CHECKED_LATER,
    seo: {
      title: 'Open-Source Plane Alternative: Helpin vs Plane',
      description: 'Compare Helpin and Plane, two open-source tools: projects, AI agents, self-hosting and pricing, and what changes when support, CRM and meetings share the work.',
    },
    hero: {
      accent: 'open-source projects with the customer included.',
      lede: 'Both are open source and both can run on your own servers. Plane is a popular project tracker with a growing AI layer. Helpin puts projects next to a support inbox, help center, CRM and meetings, so the request, the task and the follow-up share one customer history.',
    },
    glance: [
      { label: 'Customer support', competitor: 'Help desk coming soon; Intake forms on Business', helpin: 'Inbox, chat widget and help center built in' },
      { label: 'Open-source edition', competitor: 'Free-plan features; AI needs the paid edition', helpin: 'Every product feature, AI agents included' },
      { label: 'Pricing', competitor: 'Per seat, with AI credits per seat', helpin: 'One workspace price, AI usage included' },
    ],
    summary: {
      title: 'Both open source. Different scope.',
      lede: 'Plane focuses on planning and tracking the work. Helpin adds the customer side: support, CRM and meetings on the same history as the work.',
      competitor: [
        'You want a dedicated project tracker with cycles, modules, initiatives and dashboards.',
        'You need SOC 2 Type II and ISO 27001, or an air-gapped Enterprise deployment.',
        'You want native mobile apps and importers from Jira, Linear, Asana and ClickUp.',
        'You have a small team and want a free plan for up to 12 seats.',
      ],
      helpin: [
        'Customer requests arrive in your own support inbox and become tasks with the conversation attached.',
        'You want a CRM with deals and meeting notes next to your roadmap.',
        'You want AI agents, coding agents included, in the free self-hosted edition.',
        'You want one price per workspace instead of per seat.',
      ],
    },
    reasons: [
      {
        title: 'Customer requests start somewhere else',
        icon: 'requests',
        competitorLane: ['Intake form or email', 'Work item'],
        helpinLane: ['Support inbox', 'Task', 'Pull request', 'Customer told'],
        body: 'Plane collects requests through Intake, with public forms and an intake email address on the Business plan, and lists its Desk help desk as coming soon. Helpin includes the support inbox, chat widget and help center, so the customer’s conversation is already attached to the task, and the team can follow up in it when the work ships.',
      },
      {
        title: 'AI and agents need the paid edition',
        icon: 'open',
        competitorLane: ['Community: Free-plan features', 'AI: Commercial Edition'],
        helpinLane: ['Every product feature', 'AGPL-3.0'],
        body: `Plane’s Community Edition matches its Free plan. Plane AI, agents, time tracking, Customers and SSO need the closed-source Commercial Edition and a license key. Every Helpin product feature is open source under AGPL-3.0, AI agents and coding agents included. ${NO_LICENSE}`,
      },
      {
        title: 'AI is metered in credits per seat',
        icon: 'billing',
        competitorLane: ['Per seat', '+ AI credits per seat'],
        helpinLane: PRICE_LANE,
        body: 'Plane Cloud charges per seat, and Plane AI draws on monthly credits: 500 per seat on Pro and 1,000 on Business, none on Free, and no top-up. Helpin charges one price per workspace with unlimited teammates, and each Cloud plan includes an AI usage allowance, with metered overage only if you turn it on.',
      },
      {
        title: 'No CRM or meeting notes',
        icon: 'crm',
        competitorLane: ['Customers', 'Work items'],
        helpinLane: ['Contacts', 'Companies', 'Deals', 'Meetings'],
        body: 'Plane’s Customers feature, on Business, gives each customer a profile linked to their requests. Helpin adds a CRM with contacts, companies, deals and pipelines, plus meeting notes from Google Meet, Zoom, Microsoft Teams and Webex, on the same account as the work.',
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Open source and pricing',
        rows: [
          row('License', 'AGPL-3.0 for every product feature', 'AGPL-3.0 Community Edition; paid features in a closed-source Commercial Edition', ['yes', 'partial']),
          row('Self-hosted AI agents', 'Included; connect your own AI provider', 'Commercial Edition, with your own AI provider', ['yes', 'partial']),
          row('Cloud price model', 'Per workspace', 'Per seat: Pro $6, Business $13 a month billed annually'),
          row('Free plan', 'Self-hosted Community edition', 'Up to 12 seats on Cloud, and the self-hosted Community Edition'),
          row('Cloud AI', HELPIN.ai, '500 credits per seat on Pro, 1,000 on Business, none on Free'),
          row('Free trial', HELPIN.trial, '14 days of Business'),
        ],
      },
      {
        group: 'Planning',
        rows: [
          row('Roadmaps and sprints', true, 'Cycles, modules, initiatives and milestones', [undefined, 'yes']),
          row('Triage', true, 'Intake; forms and email on Business', [undefined, 'yes']),
          row('GitHub and GitLab', 'PR and MR linking', 'On Pro and above', ['yes', 'yes']),
          row('Coding agents', HELPIN.coding, 'Assign work items to Cursor on Pro and above', ['yes', 'yes']),
          row('Native mobile apps', false, 'iOS and Android', [undefined, 'yes']),
        ],
      },
      {
        group: 'Customers',
        rows: [
          row('Support inbox and live chat', HELPIN.channels, 'Help desk coming soon', ['yes', 'no']),
          row('Help center', HELPIN.helpCenter, 'Published pages on Pro', ['yes', 'partial']),
          row('Customer profiles', 'Contacts and companies with every conversation and task', 'Customers on Business', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, false, ['yes']),
          row('Meeting notes', HELPIN.meetings, 'Search Granola notes through Plane AI', ['yes', 'partial']),
          row('MCP server', HELPIN.mcp, 'Hosted for Plane Cloud, or run locally', ['yes', 'yes']),
        ],
      },
    ],
    strengths: [
      { icon: 'speed', title: 'Planning depth.', body: 'Cycles, modules, epics, initiatives, milestones, dashboards and estimates, with list, board, calendar, Gantt and spreadsheet layouts.' },
      { icon: 'community', title: 'Adoption.', body: 'Plane says more than 50,000 teams use it, and its repository has about 60,000 GitHub stars.' },
      { icon: 'enterprise', title: 'Compliance and deployment.', body: 'SOC 2 Type II and ISO 27001:2022, SAML and OIDC, Kubernetes, and an air-gapped edition on Enterprise Grid.' },
      { icon: 'mobile', title: 'Mobile and imports.', body: 'Native iOS, Android and desktop apps, and importers for Jira, Linear, Asana and ClickUp.' },
    ],
    calculator: {
      seatsLabel: 'People on the team',
      seatsUnit: ['person', 'people'],
      seats: 20,
      plans: [{ name: 'Pro', annual: 6, monthly: 8 }, { name: 'Business', annual: 13, monthly: 15 }],
      plan: 1,
      helpinPlan: 'growth',
      notes: [
        'Business is the Plane plan with Customers and intake forms. Plane AI uses monthly credits included per seat, with no top-up, so AI isn’t priced here.',
        'For small teams on Pro, Plane costs less than Helpin, and its free plan covers up to 12 seats.',
      ],
    },
    switching: {
      title: 'Moving from Plane.',
      lede: 'You don’t have to move everything at once. Many teams start with support.',
      take: ['Work items, as CSV, Excel or JSON', 'Pages you want to keep as docs', 'Cycles and modules you want to plan in Helpin'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Plane connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Plane, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Plane Cloud’s tools through its hosted MCP server, so engineering can stay in Plane while support runs in Helpin.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Plane work items', status: 'Not yet', body: 'There is no Plane importer yet. Plane exports work items as CSV, Excel or JSON, and Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      ['Is Helpin a Plane alternative?', 'For teams that want projects, support and CRM in one open-source product, yes. If you want a dedicated project tracker with deep planning features, Plane is a strong choice.'],
      ['Are Helpin and Plane both open source?', 'Yes. Plane’s Community Edition is AGPL-3.0 and matches its Free plan; its paid features run in a closed-source Commercial Edition. Every Helpin product feature is open source under AGPL-3.0.'],
      ['Which is better for self-hosting?', 'Plane has more deployment options, including Kubernetes and an air-gapped Enterprise edition. Helpin’s free Community edition includes AI agents, support, projects and CRM, and is currently a 0.2 beta.'],
      ['Does Plane have a help desk?', 'Plane lists its Desk help desk as coming soon. Today it collects requests through Intake, with public forms and email on Business, and links them to customer profiles with Customers on Business.'],
      ['Can Helpin import from Plane?', 'Not yet. Plane exports work items as CSV, Excel or JSON, and Helpin imports projects from Shortcut today.'],
      ...sharedFaqs('Plane', { selfHost: false }),
    ],
    closing: { title: 'Open source, with the customer in the loop.', description: 'Self-host the Community edition for free, or start a 14-day trial of Helpin Cloud with no card.' },
    sources: [
      { label: 'Plane pricing', url: 'https://plane.so/pricing' },
      { label: 'Plane billing and plans', url: 'https://docs.plane.so/workspaces-and-users/billing-and-plans' },
      { label: 'Plane AI credits', url: 'https://docs.plane.so/ai/plane-ai-credits' },
      { label: 'Plane self-hosted editions', url: 'https://developers.plane.so/self-hosting/editions-and-versions' },
      { label: 'Plane self-hosting 101', url: 'https://developers.plane.so/self-hosting/self-hosting-101' },
      { label: 'Plane license', url: 'https://github.com/makeplane/plane/blob/preview/LICENSE.txt' },
      { label: 'Plane Customers', url: 'https://docs.plane.so/customers' },
      { label: 'Plane home (Desk coming soon)', url: 'https://plane.so/' },
      { label: 'Plane and Cursor', url: 'https://docs.plane.so/integrations/cursor' },
      { label: 'Plane MCP server', url: 'https://developers.plane.so/dev-tools/mcp-server' },
      { label: 'Plane importers', url: 'https://docs.plane.so/importers/overview' },
      { label: 'Plane export', url: 'https://docs.plane.so/core-concepts/export' },
      { label: 'Plane compliance', url: 'https://plane.so/blog/plane-wins-all-top-compliance-certifications' },
      { label: 'Plane air-gapped requirements', url: 'https://developers.plane.so/self-hosting/methods/airgapped-requirements' },
      { label: 'Plane open source', url: 'https://plane.so/open-source' },
    ],
  },
  {
    slug: 'jira',
    name: 'Jira',
    group: 'Project management',
    category: 'Project management',
    cardLine: 'A widely used issue tracker with support, feedback and meeting notes sold as separate products, compared with one workspace for all of it.',
    checked: CHECKED_LATER,
    seo: {
      title: 'Open-Source Jira Alternative: Helpin vs Jira',
      description: 'Compare Helpin and Jira: projects, AI agents, support and pricing, and what changes when your support inbox, CRM and meetings live alongside the work.',
    },
    hero: {
      accent: 'the customer’s request and the work, in one place.',
      lede: 'Jira is a widely used issue tracker with a large app ecosystem; Atlassian sells its help desk, product feedback and meeting notes as separate products. Helpin puts projects, a support inbox, help center, CRM and meetings in one open-source workspace, so the request, the task and the follow-up share one customer history.',
    },
    glance: [
      { label: 'Customer support', competitor: 'A separate product, priced per agent', helpin: 'Inbox, chat widget and help center built in' },
      { label: 'Self-hosting', competitor: 'Data Center closed to new customers', helpin: 'Open source: Cloud or your servers' },
      { label: 'AI', competitor: 'Rovo credits per user, overage billed from December', helpin: 'AI usage included in the workspace price' },
    ],
    summary: {
      title: 'A tracker for the work, or one workspace for the customer and the work.',
      lede: 'Jira is built to plan and track work at any scale. Helpin is for teams that want customer requests, the work behind them and the follow-up in one product.',
      competitor: [
        'You run large engineering programs that need advanced planning, approvals and many sites.',
        'You depend on the Atlassian ecosystem: Confluence, Bitbucket and more than 4,000 Marketplace apps.',
        'You want Rovo and a wide choice of coding agents, including Claude, Cursor and GitHub Copilot.',
        'You have a small team: Jira’s free plan covers up to 10 users.',
      ],
      helpin: [
        'Support questions become tasks with the customer conversation attached.',
        'You want support, CRM and meeting notes in the same product as your roadmap.',
        'You want to run on your own servers with an open-source product.',
        'You want one workspace price with AI usage included.',
      ],
    },
    reasons: [
      {
        title: 'Support is a separate product',
        icon: 'requests',
        competitorLane: ['Service Collection', 'Linked work item'],
        helpinLane: ['Support inbox', 'Task', 'Pull request', 'Customer told'],
        body: 'Jira points help-desk teams to Jira Service Management, which is now sold inside Atlassian’s Service Collection and priced per agent. In Helpin the support inbox, chat widget and help center are part of the product, so the conversation is already attached to the task, and the team can follow up in it when the work ships.',
      },
      {
        title: 'Self-hosting is winding down',
        icon: 'hosting',
        competitorLane: ['Data Center', 'Read-only in March 2029'],
        helpinLane: ['Helpin Cloud', 'or your servers'],
        body: `Atlassian stopped selling Data Center to new customers on March 30, 2026, and Data Center products become read-only on March 28, 2029. Helpin is open source under AGPL-3.0: use Helpin Cloud, or run the Community edition on your own servers with Docker Compose. ${NO_LICENSE}`,
      },
      {
        title: 'AI is metered in credits',
        icon: 'billing',
        competitorLane: ['Per user', '+ Rovo credits'],
        helpinLane: PRICE_LANE,
        body: 'Paid Jira plans include Rovo credits per user each month: 25 on Standard, 70 on Premium and 150 on Enterprise. From December 3, 2026, extra usage is billed at $0.01 a credit by default, and Rovo Dev is a separate $20 per developer a month. Helpin charges one price per workspace, and each Cloud plan includes an AI usage allowance, with metered overage only if you turn it on.',
      },
      {
        title: 'No CRM, and meeting notes cost extra',
        icon: 'crm',
        competitorLane: ['Jira', 'Loom', 'Your CRM'],
        helpinLane: ['Contacts', 'Companies', 'Deals', 'Meetings'],
        body: 'Atlassian doesn’t offer a CRM, and automatic meeting notes come with Loom’s Business + AI plan. Helpin includes contacts, companies, deals and pipelines, plus a meeting notetaker for Google Meet, Zoom, Microsoft Teams and Webex, on the same account as the work.',
      },
    ],
    tableLede: 'A check means included, a dash means partly or on some plans, and a cross means not available.',
    table: [
      {
        group: 'Pricing',
        rows: [
          row('Price model', 'Per workspace', 'Per user: Standard $9.05, Premium $18.30 a month billed monthly, for up to 100 users'),
          row('Free plan', 'Self-hosted Community edition', 'Up to 10 users'),
          row('AI', HELPIN.ai, 'Rovo credits per user; extra usage billed from December 3, 2026'),
          row('Support inbox', 'Included', 'Service Collection: free for 3 agents, then $25 an agent a month for up to 15'),
          row('Open source and self-hosting', HELPIN.selfHost, 'Data Center closed to new customers', ['yes', 'no']),
        ],
      },
      {
        group: 'Planning',
        rows: [
          row('Roadmaps and sprints', true, 'Boards and sprints; advanced planning on Premium', [undefined, 'yes']),
          row('GitHub and GitLab', 'PR and MR linking', 'GitHub, GitLab and Bitbucket', ['yes', 'yes']),
          row('Coding agents', HELPIN.coding, 'Rovo, Jira Coding Agent, Claude, Cursor and GitHub Copilot', ['yes', 'yes']),
          row('App marketplace', 'MCP connections and web SDKs', 'More than 4,000 Marketplace apps', ['partial', 'yes']),
          row('Native mobile apps', false, 'iOS and Android', [undefined, 'yes']),
        ],
      },
      {
        group: 'Customers',
        rows: [
          row('Support inbox and live chat', HELPIN.channels, 'Service Collection, priced per agent', ['yes', 'partial']),
          row('Help center', HELPIN.helpCenter, 'Knowledge base with Service Collection and Confluence', ['yes', 'partial']),
          row('CRM with deals', HELPIN.crm, false, ['yes']),
          row('Meeting notes', HELPIN.meetings, 'Loom Business + AI, a separate product', ['yes', 'partial']),
          row('MCP server', HELPIN.mcp, 'Rovo MCP Server', ['yes', 'yes']),
        ],
      },
    ],
    strengths: [
      { icon: 'ecosystem', title: 'Ecosystem.', body: 'More than 4,000 Marketplace apps, and close links to Confluence, Bitbucket, GitHub and GitLab.' },
      { icon: 'enterprise', title: 'Scale and controls.', body: 'Advanced planning, capacity management, approvals, sandboxes and uptime SLAs on Premium, and multiple sites and analytics on Enterprise.' },
      { icon: 'agents', title: 'Agent choice.', body: 'Rovo agents, the Jira Coding Agent and third-party coding agents such as Claude, Cursor and GitHub Copilot, plus an official MCP server.' },
      { icon: 'mobile', title: 'Mobile and imports.', body: 'Native iOS and Android apps, and importers for Asana, monday, ClickUp, Trello, Linear, GitHub and more.' },
    ],
    calculator: {
      seatsLabel: 'People on the team',
      seatsUnit: ['person', 'people'],
      seats: 20,
      plans: [{ name: 'Standard', annual: 7.54, monthly: 9.05 }, { name: 'Premium', annual: 15.25, monthly: 18.30 }],
      plan: 0,
      helpinPlan: 'growth',
      notes: [
        'Jira prices annual plans by user tier; the annual rates shown are the 100-user tier. Atlassian raises these list prices on October 13, 2026.',
        'Jira doesn’t include a support inbox. Service Collection is free for 3 agents, then $25 an agent a month for up to 15; Loom and Confluence are priced separately too.',
      ],
    },
    switching: {
      title: 'Moving from Jira.',
      lede: 'You don’t have to move everything at once. Many teams start with support.',
      take: ['Work items, as CSV, Excel or XML', 'Projects and sprints you want to plan in Helpin', 'Confluence pages you want to keep as docs'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Jira connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Jira, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Jira’s tools through Atlassian’s Rovo MCP Server, so engineering can stay in Jira while support runs in Helpin. Your Atlassian admin may need to allow Helpin’s domain.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Jira work items', status: 'Not yet', body: 'There is no Jira importer yet. Jira exports work items as CSV, Excel or XML, and Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      ['Is Helpin a Jira alternative?', 'For teams that want projects, support and CRM in one open-source product, yes. If you run large engineering programs that depend on advanced planning and the Atlassian ecosystem, Jira is a strong choice.'],
      ['Can I still self-host Jira?', 'Atlassian stopped selling Data Center to new customers on March 30, 2026. Existing customers can buy until March 30, 2028, and Data Center products become read-only on March 28, 2029. Helpin’s Community edition is open source and runs on your own servers.'],
      ['Does Jira include a help desk?', 'No. Atlassian points help-desk teams to Jira Service Management, now part of its Service Collection and priced per agent, with a free plan for 3 agents. Helpin includes the support inbox, chat widget and help center.'],
      ['How does Jira bill for AI?', 'Paid Jira plans include Rovo credits per user each month: 25 on Standard, 70 on Premium and 150 on Enterprise. From December 3, 2026, extra usage is billed at $0.01 a credit, and that’s on by default. Rovo Dev is priced separately at $20 per developer a month.'],
      ['Can Helpin import from Jira?', 'Not yet. Jira exports work items as CSV, Excel or XML, and Helpin’s agents can use Jira’s tools through MCP while you move.'],
      ...sharedFaqs('Jira').filter(([question]) => !question.startsWith('Can we run Helpin alongside')),
    ],
    closing: { title: 'Keep the customer next to the work.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    sources: [
      { label: 'Jira pricing', url: 'https://www.atlassian.com/software/jira/pricing' },
      { label: 'Atlassian cloud price tables (current and from October 13, 2026)', url: 'https://www.atlassian.com/licensing/future-pricing/cloud/list/pricing-tables' },
      { label: 'Atlassian future pricing FAQ', url: 'https://www.atlassian.com/licensing/future-pricing/cloud/list/faqs' },
      { label: 'Rovo usage limits and extra usage billing', url: 'https://support.atlassian.com/rovo/docs/rovo-usage-limits/' },
      { label: 'Rovo Dev pricing', url: 'https://www.atlassian.com/software/rovo-dev/pricing' },
      { label: 'Agents in Jira', url: 'https://support.atlassian.com/jira-software-cloud/docs/collaborate-on-work-items-with-ai-agents/' },
      { label: 'Jira Coding Agent repositories', url: 'https://support.atlassian.com/jira-software-cloud/docs/supported-repositories-for-jira-coding-agent/' },
      { label: 'Rovo MCP Server', url: 'https://www.atlassian.com/platform/rovo-mcp' },
      { label: 'Data Center end of life', url: 'https://www.atlassian.com/licensing/data-center-end-of-life' },
      { label: 'Jira email and help desks', url: 'https://support.atlassian.com/jira-cloud-administration/docs/create-issues-and-comments-from-email/' },
      { label: 'Service Collection pricing', url: 'https://www.atlassian.com/collections/service/pricing' },
      { label: 'Customer Service Management', url: 'https://www.atlassian.com/software/customer-service-management' },
      { label: 'Loom pricing', url: 'https://www.atlassian.com/software/loom/pricing' },
      { label: 'Atlassian Marketplace', url: 'https://www.atlassian.com/software/marketplace' },
      { label: 'Jira importers', url: 'https://support.atlassian.com/jira-software-cloud/docs/import-data-into-jira/' },
      { label: 'Jira export', url: 'https://support.atlassian.com/jira-software-cloud/docs/export-search-results/' },
    ],
  },
];
