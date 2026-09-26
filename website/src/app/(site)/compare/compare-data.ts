// Comparison page content. One entry per competitor; the /compare/[slug] template renders it.
// Competitor facts come from their public pages on the `checked` date (see `sources`).
// Helpin facts must match the shipped product and /pricing. Re-check at least quarterly.
import type { PageSeo } from '../../../lib/metadata.ts';

export type Cell = boolean | string;
type FAQ = readonly [question: string, answer: string];

export type Competitor = {
  slug: string;
  name: string;
  category: string;
  cardLine: string;
  checked: string;
  seo: { title: string; description: string };
  hero: { title: string; lede: string };
  summary: { title: string; lede: string; competitor: string[]; helpin: string[] };
  tableLede: string;
  table: { group: string; rows: { label: string; helpin: Cell; competitor: Cell }[] }[];
  differencesTitle: string;
  differences: { title: string; body: string }[];
  strengths: { title: string; body: string }[];
  cost?: {
    title: string;
    lede: string;
    helpin: { amount: string; detail: string };
    competitor: { amount: string; detail: string };
    note: string;
  };
  switching: { title: string; body: string[] };
  faqs: FAQ[];
  closing: { title: string; description: string };
  sources: { label: string; url: string }[];
};

export function formatChecked(date: string) {
  return new Date(`${date}T00:00:00Z`).toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });
}

export function competitorSeo(competitor: Competitor): PageSeo {
  return {
    ...competitor.seo,
    canonicalPath: `/compare/${competitor.slug}`,
    imagePath: `/og/helpin-compare-${competitor.slug}-green-v4.png`,
    imageAlt: `Helpin vs ${competitor.name}`,
  };
}

const CHECKED = '2026-09-26';

// Shared Helpin facts, so every page states them the same way.
const HELPIN = {
  seats: 'Unlimited teammates on every plan',
  ai: 'AI usage allowance included in every Cloud plan',
  selfHost: 'Free Community edition (AGPL-3.0, 0.1 beta)',
  trial: '14 days, no card',
  channels: 'Web chat and email',
  sdks: 'Web: JavaScript, React, Next.js, Vue',
  helpCenter: 'Custom domain, AI answers, API reference',
  projects: 'Roadmaps, sprints, objectives, epics',
  crm: 'Contacts, companies, deals',
  meetings: 'Meet, Zoom, Teams and Webex',
  coding: 'Opens GitHub PRs and GitLab MRs for review',
  mcp: 'Hosted and self-hosted (beta)',
};

const SWITCH_NOTE = 'Most teams start Helpin alongside their current tool: install the widget on a few pages, connect a support address, and move queues over when the team is ready.';

export const COMPETITORS: Competitor[] = [
  {
    slug: 'intercom',
    name: 'Intercom',
    category: 'AI customer support',
    cardLine: 'Per-seat plans plus a fee per AI outcome, compared with one workspace price and connected product work.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Intercom: an open-source alternative compared',
      description: 'Compare Helpin and Intercom on AI support, pricing and self-hosting. Helpin adds projects, CRM and meetings, with one price per workspace and no seat fees.',
    },
    hero: {
      title: 'Helpin vs Intercom: when the answer needs a fix.',
      lede: 'Both answer customers with AI. Intercom is a mature support platform with wide channel coverage. Helpin keeps support next to your projects, CRM, meetings and docs, so the question, the fix and the follow-up stay together. You can also self-host it.',
    },
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
        'You want AI agents that can plan the fix and open a pull request for review.',
        'You want one price per workspace instead of per seat and per AI outcome.',
        'You want the option to self-host an open-source product.',
      ],
    },
    tableLede: 'Checkmarks mean the capability is available. Where a plan matters, it is noted.',
    table: [
      {
        group: 'Pricing',
        rows: [
          { label: 'Price model', helpin: 'Per workspace', competitor: 'Per seat: $29, $85 or $132 a month billed annually' },
          { label: 'Teammates', helpin: HELPIN.seats, competitor: 'Each full seat is paid; Lite seats on higher plans' },
          { label: 'AI agent', helpin: HELPIN.ai, competitor: 'Fin at $0.99 per outcome' },
          { label: 'Free trial', helpin: HELPIN.trial, competitor: '14 days, no card' },
          { label: 'Open source and self-hosting', helpin: HELPIN.selfHost, competitor: false },
        ],
      },
      {
        group: 'Support',
        rows: [
          { label: 'Channels', helpin: HELPIN.channels, competitor: 'Chat, email, phone, WhatsApp, SMS, social, Slack and more' },
          { label: 'AI answers with human handoff', helpin: true, competitor: true },
          { label: 'Help center', helpin: HELPIN.helpCenter, competitor: 'Public help center; multilingual on Advanced' },
          { label: 'Widget SDKs', helpin: HELPIN.sdks, competitor: 'Web, iOS, Android, React Native' },
          { label: 'SSO and SLA policies', helpin: false, competitor: 'Expert plan' },
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          { label: 'Project management', helpin: HELPIN.projects, competitor: 'Through integrations such as Jira' },
          { label: 'CRM with deals', helpin: HELPIN.crm, competitor: 'Contacts and companies; deals through integrations' },
          { label: 'Meeting notes', helpin: HELPIN.meetings, competitor: false },
          { label: 'Coding agents', helpin: HELPIN.coding, competitor: false },
          { label: 'MCP server', helpin: HELPIN.mcp, competitor: 'Hosted' },
        ],
      },
    ],
    differencesTitle: 'Where the two products take different paths.',
    differences: [
      {
        title: 'What happens after the reply',
        body: 'Intercom focuses on resolving the conversation, and hands product work to tools like Jira. In Helpin, a support conversation can become a task on the roadmap. The history stays attached, so the team and its agents can follow up when the fix ships.',
      },
      {
        title: 'How AI is billed',
        body: 'Intercom charges $0.99 for each Fin outcome, on top of seats. Helpin includes an AI usage allowance in each Cloud plan. On an active paid plan you can turn on metered overage at the published rates, and self-hosted installs use your own AI provider.',
      },
      {
        title: 'Who can use it',
        body: 'Intercom prices by seat, so each full teammate adds cost. Helpin charges per workspace with unlimited teammates, which makes it easier to give engineering, sales and success access to the same customer history.',
      },
      {
        title: 'Where it runs',
        body: 'Intercom is a hosted service. Helpin is open source under AGPL-3.0. Use Helpin Cloud, or run the Community edition on your own infrastructure with Docker Compose.',
      },
    ],
    strengths: [
      { title: 'Channels.', body: 'Phone and voice AI, WhatsApp, SMS, social channels, Slack, Discord and Microsoft Teams. Helpin supports web chat and email.' },
      { title: 'Ecosystem.', body: 'More than 450 apps and integrations, and native mobile SDKs for iOS and Android.' },
      { title: 'Maturity.', body: 'Over 30,000 customers and years of support-specific features, including SLAs, SSO and custom reports.' },
      { title: 'Proactive messaging.', body: 'Product tours, surveys and targeted in-app messages through add-ons.' },
    ],
    cost: {
      title: 'What a team of eight might pay.',
      lede: 'List prices for eight support teammates, billed annually. Channel usage and add-ons are excluded.',
      helpin: { amount: '$239 a month', detail: 'Growth plan for the whole workspace, including custom agents, automation and AI routing, with $239 of AI usage included each month. The price stays the same for 8 or 80 teammates.' },
      competitor: { amount: '$680 a month + AI', detail: 'Advanced plan at $85 a seat. Fin adds $0.99 per outcome, so 1,000 AI outcomes a month would bring the total to about $1,670.' },
      note: 'Helpin AI usage is measured in tokens rather than outcomes, so the cost of 1,000 conversations depends on their length and the model used.',
    },
    switching: {
      title: 'Moving from Intercom.',
      body: [
        'An importer for Intercom conversations and contacts is coming soon. Until then, Intercom can export conversations as CSV or through its API, and help articles can be recreated in Helpin Knowledge.',
        SWITCH_NOTE,
      ],
    },
    faqs: [
      ['Is Helpin a good Intercom alternative?', 'It is if your team wants AI support connected to product work, CRM and meetings, or wants to self-host. If you depend on phone, WhatsApp or social channels, or on native mobile SDKs, Intercom covers more today.'],
      ['Is Helpin cheaper than Intercom?', 'For most teams with several support teammates, yes. Helpin charges one price per workspace with AI usage included. Intercom charges per seat plus $0.99 per Fin outcome. Compare both with your own team size and volume.'],
      ['Is Intercom now called Fin?', 'The company renamed itself Fin in May 2026 and was acquired by Salesforce in September 2026. The helpdesk product is still called Intercom.'],
      ['Can Helpin import my Intercom data?', 'An Intercom importer is coming soon. You can export conversations from Intercom today and start Helpin alongside it while you move over.'],
    ],
    closing: { title: 'Bring support and the work behind it together.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    sources: [
      { label: 'Intercom plans explained', url: 'https://www.intercom.com/help/en/articles/9061614-fin-and-intercom-plans-explained' },
      { label: 'Intercom seats', url: 'https://www.intercom.com/help/en/articles/8205716-seats' },
      { label: 'Fin AI agent outcomes', url: 'https://www.intercom.com/help/en/articles/8205718-fin-ai-agent-outcomes' },
      { label: 'Intercom pricing FAQ', url: 'https://www.intercom.com/help/en/articles/8344190-pricing-faqs' },
      { label: 'Intercom free trial', url: 'https://www.intercom.com/help/en/articles/891-how-to-start-a-free-trial-of-fin-and-intercom' },
      { label: 'Intercom mobile SDK FAQ', url: 'https://www.intercom.com/help/en/articles/7669340-mobile-sdk-faqs' },
      { label: 'Intercom MCP server', url: 'https://developers.intercom.com/docs/guides/mcp' },
      { label: 'Salesforce completes acquisition of Fin', url: 'https://investor.salesforce.com/news/news-details/2026/Salesforce-Completes-Acquisition-of-Fin/default.aspx' },
    ],
  },
  {
    slug: 'zendesk',
    name: 'Zendesk',
    category: 'Help desk',
    cardLine: 'An established omnichannel help desk, compared with support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Zendesk: features, pricing and AI compared',
      description: 'Compare Helpin and Zendesk on AI support, pricing and self-hosting. See where Zendesk leads and where Helpin connects support to projects and CRM.',
    },
    hero: {
      title: 'Helpin vs Zendesk: one history from ticket to release.',
      lede: 'Zendesk is an established help desk with deep omnichannel and contact center features. Helpin is a newer, open-source platform that keeps support, projects, CRM, meetings and docs on one customer history, with AI agents working across them.',
    },
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
        'You want one price per workspace, with unlimited teammates.',
        'You want to self-host an open-source product.',
      ],
    },
    tableLede: 'Checkmarks mean the capability is available. Where a plan matters, it is noted.',
    table: [
      {
        group: 'Pricing',
        rows: [
          { label: 'Price model', helpin: 'Per workspace', competitor: 'Per agent: Suite Team $55, Professional $115 a month billed annually' },
          { label: 'Teammates', helpin: HELPIN.seats, competitor: 'Each agent is paid' },
          { label: 'AI agent', helpin: HELPIN.ai, competitor: 'Billed per verified resolution, beyond a small included allowance' },
          { label: 'Free trial', helpin: HELPIN.trial, competitor: '14 days, no card' },
          { label: 'Open source and self-hosting', helpin: HELPIN.selfHost, competitor: false },
        ],
      },
      {
        group: 'Support',
        rows: [
          { label: 'Channels', helpin: HELPIN.channels, competitor: 'Email, messaging, chat, voice with IVR, social' },
          { label: 'AI answers with human handoff', helpin: true, competitor: true },
          { label: 'Help center', helpin: HELPIN.helpCenter, competitor: 'Knowledge base on Suite plans' },
          { label: 'Widget SDKs', helpin: HELPIN.sdks, competitor: 'Web, iOS, Android' },
          { label: 'SSO and SLA policies', helpin: false, competitor: true },
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          { label: 'Project management', helpin: HELPIN.projects, competitor: 'Through integrations such as Jira' },
          { label: 'CRM with deals', helpin: HELPIN.crm, competitor: 'Zendesk Sell, retiring in August 2027' },
          { label: 'Meeting notes', helpin: HELPIN.meetings, competitor: false },
          { label: 'Coding agents', helpin: HELPIN.coding, competitor: false },
          { label: 'MCP server', helpin: HELPIN.mcp, competitor: 'Announced; MCP client in early access' },
        ],
      },
    ],
    differencesTitle: 'Where the two products take different paths.',
    differences: [
      {
        title: 'Tickets and the work behind them',
        body: 'Zendesk manages the ticket and connects to tools like Jira for engineering work. Helpin keeps the conversation, the project task, the pull request and the customer follow-up in one place, so nobody has to copy context between systems.',
      },
      {
        title: 'Pricing as the team grows',
        body: 'Zendesk charges per agent, and add-ons such as Copilot are priced per agent too. Helpin charges per workspace with unlimited teammates, and includes an AI usage allowance in each Cloud plan.',
      },
      {
        title: 'Customer relationships',
        body: 'Zendesk plans to retire Zendesk Sell in August 2027 and stop offering a sales CRM. Helpin includes contacts, companies and deals, with the support and meeting history attached to each account.',
      },
      {
        title: 'Setup and ownership',
        body: 'Zendesk is a hosted service with a deep admin surface. Helpin is open source under AGPL-3.0: use Helpin Cloud or run it yourself with Docker Compose.',
      },
    ],
    strengths: [
      { title: 'Omnichannel and voice.', body: 'Native contact center, IVR and messaging channels. Helpin supports web chat and email.' },
      { title: 'Enterprise controls.', body: 'SSO, SLA policies, sandboxes, custom roles and approval workflows.' },
      { title: 'Ecosystem.', body: 'More than 1,800 marketplace apps and mobile SDKs for iOS and Android.' },
      { title: 'Reporting.', body: 'Mature dashboards and analytics for large service teams.' },
    ],
    cost: {
      title: 'What a team of eight might pay.',
      lede: 'List prices for eight support agents, billed annually. Add-ons and AI resolutions beyond the included allowance are excluded.',
      helpin: { amount: '$239 a month', detail: 'Growth plan for the whole workspace, including custom agents, automation and AI routing, with $239 of AI usage included each month. The price stays the same for 8 or 80 teammates.' },
      competitor: { amount: '$440 a month + AI', detail: 'Suite Team at $55 an agent. Verified AI resolutions beyond the included allowance are billed separately, and Copilot is $50 per agent.' },
      note: 'Zendesk does not publish its price per verified resolution, so we have not estimated it.',
    },
    switching: {
      title: 'Moving from Zendesk.',
      body: [
        'An importer for Zendesk tickets and contacts is coming soon. Zendesk account owners can request ticket and user exports today, and help articles can be recreated in Helpin Knowledge.',
        SWITCH_NOTE,
      ],
    },
    faqs: [
      ['Is Helpin a good Zendesk alternative?', 'It is for product-led teams that want support connected to projects, CRM and meetings, or that want to self-host. Large contact centers that need voice, IVR and many channels are better served by Zendesk today.'],
      ['Is Helpin cheaper than Zendesk?', 'For most teams with several agents, yes. Helpin charges one price per workspace with an AI allowance included. Zendesk charges per agent, with AI resolutions and Copilot billed separately.'],
      ['What happens to Zendesk Sell?', 'Zendesk has announced that Zendesk Sell will be retired on August 31, 2027. Helpin includes a CRM with contacts, companies and deals.'],
      ['Can Helpin import my Zendesk data?', 'A Zendesk importer is coming soon. You can export data from Zendesk today and start Helpin alongside it while you move over.'],
    ],
    closing: { title: 'Take the ticket all the way to the release.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
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
    category: 'Shared inbox',
    cardLine: 'A simple shared inbox priced per user, compared with support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Help Scout: features and pricing compared',
      description: 'Compare Helpin and Help Scout on shared inbox, AI answers, pricing and self-hosting, and see when projects, CRM and meetings in one place matter.',
    },
    hero: {
      title: 'Helpin vs Help Scout: a simple inbox, and what comes after it.',
      lede: 'Help Scout is a well-loved shared inbox that keeps support simple. Helpin covers the inbox too, and adds projects, CRM, meetings and AI agents on the same customer history, with one price per workspace.',
    },
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
        'You want AI usage included instead of paying per resolution.',
        'You want to self-host an open-source product.',
      ],
    },
    tableLede: 'Checkmarks mean the capability is available. Where a plan matters, it is noted.',
    table: [
      {
        group: 'Pricing',
        rows: [
          { label: 'Price model', helpin: 'Per workspace', competitor: 'Per user: Standard $25, Plus $45 a month billed annually' },
          { label: 'Free plan', helpin: 'Self-hosted Community edition', competitor: 'Up to 5 users and 100 contacts a month' },
          { label: 'AI agent', helpin: HELPIN.ai, competitor: 'AI Answers at $0.75 per resolution' },
          { label: 'Free trial', helpin: HELPIN.trial, competitor: '15 days, no card' },
          { label: 'Open source and self-hosting', helpin: HELPIN.selfHost, competitor: false },
        ],
      },
      {
        group: 'Support',
        rows: [
          { label: 'Channels', helpin: HELPIN.channels, competitor: 'Email, chat, WhatsApp, Instagram, Messenger, SMS' },
          { label: 'AI answers with human handoff', helpin: true, competitor: true },
          { label: 'Help center', helpin: HELPIN.helpCenter, competitor: 'Docs sites; 2 on Standard, 3 on Plus' },
          { label: 'Widget SDKs', helpin: HELPIN.sdks, competitor: 'Web, iOS, Android' },
          { label: 'SLA policies', helpin: false, competitor: 'Limited on Standard and Plus' },
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          { label: 'Project management', helpin: HELPIN.projects, competitor: 'Through the Jira integration on Plus' },
          { label: 'CRM with deals', helpin: HELPIN.crm, competitor: 'Customer properties and company profiles' },
          { label: 'Meeting notes', helpin: HELPIN.meetings, competitor: false },
          { label: 'Coding agents', helpin: HELPIN.coding, competitor: false },
          { label: 'MCP server', helpin: HELPIN.mcp, competitor: 'Hosted, read-only' },
        ],
      },
    ],
    differencesTitle: 'Where the two products take different paths.',
    differences: [
      {
        title: 'Beyond the inbox',
        body: 'Help Scout keeps its focus on conversations and docs, and connects to other tools for the rest. Helpin adds projects, CRM and meeting notes, so a request can become tracked work with the customer history attached.',
      },
      {
        title: 'How AI is billed',
        body: 'Help Scout charges $0.75 for each AI resolution, on top of user fees. Helpin includes an AI usage allowance in each Cloud plan, and self-hosted installs use your own AI provider.',
      },
      {
        title: 'Growing the team',
        body: 'Help Scout charges per user. Helpin charges per workspace with unlimited teammates, so engineers and account managers can join without changing the price.',
      },
      {
        title: 'Where it runs',
        body: 'Help Scout is a hosted service. Helpin is open source under AGPL-3.0: use Helpin Cloud or run it on your own infrastructure.',
      },
    ],
    strengths: [
      { title: 'Simplicity.', body: 'A calm, email-like inbox that reviewers consistently praise for ease of use.' },
      { title: 'Channels.', body: 'WhatsApp, Instagram, Messenger and SMS on paid plans. Helpin supports web chat and email.' },
      { title: 'Free plan and migration.', body: 'A free plan for up to five users, and a built-in importer for conversations from Zendesk, Intercom and more than 30 other tools.' },
      { title: 'Mobile.', body: 'Beacon SDKs for iOS and Android.' },
    ],
    cost: {
      title: 'What a team of eight might pay.',
      lede: 'List prices for eight support teammates, billed annually.',
      helpin: { amount: '$239 a month', detail: 'Growth plan for the whole workspace, including custom agents, automation and AI routing, with $239 of AI usage included each month.' },
      competitor: { amount: '$360 a month + AI', detail: 'Plus plan at $45 a user. AI Answers add $0.75 per resolution, so 300 AI resolutions a month would bring the total to about $585.' },
      note: 'For a smaller budget, Helpin Starter is $79 a month billed annually, and Help Scout Standard is $25 a user.',
    },
    switching: {
      title: 'Moving from Help Scout.',
      body: [
        'Helpin can import your Help Scout Docs articles today. Conversation history is not imported yet.',
        SWITCH_NOTE,
      ],
    },
    faqs: [
      ['Is Helpin a good Help Scout alternative?', 'It is if you want support connected to projects, CRM and meetings, AI usage included in the price, or the option to self-host. If you mainly need a simple inbox with social channels, Help Scout covers that well.'],
      ['Does Help Scout charge per contact?', 'Help Scout moved to contact-based billing in 2024 and later returned to per-user pricing for new customers. Some existing accounts still use contact-based billing.'],
      ['Can Helpin import from Help Scout?', 'Yes, for Help Scout Docs articles. Conversation history is not imported yet.'],
      ['Is Helpin cheaper than Help Scout?', 'It depends on team size. Help Scout can cost less for one or two users, especially on its free plan. With more teammates or regular AI resolutions, Helpin’s single workspace price usually costs less.'],
    ],
    closing: { title: 'Keep support simple, and connect what comes next.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
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
    category: 'Open-source support',
    cardLine: 'Two open-source options: a multichannel inbox, or support connected to projects, CRM and meetings.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Chatwoot: open-source support platforms compared',
      description: 'Compare Helpin and Chatwoot, two open-source customer support platforms: channels, AI, self-hosting costs, and projects, CRM and meetings.',
    },
    hero: {
      title: 'Helpin vs Chatwoot: two open-source ways to run support.',
      lede: 'Both are open source and both can run on your own servers. Chatwoot is a mature, multichannel inbox. Helpin is a newer platform that connects support to projects, CRM, meetings and docs, with AI agents working across them.',
    },
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
      ],
    },
    tableLede: 'Checkmarks mean the capability is available. Where a plan or edition matters, it is noted.',
    table: [
      {
        group: 'Open source and pricing',
        rows: [
          { label: 'License', helpin: 'AGPL-3.0 for every product feature', competitor: 'MIT core, with a separate paid enterprise edition' },
          { label: 'Self-hosted AI agent', helpin: 'Included; connect your own AI provider', competitor: 'Captain needs a paid plan from $19 an agent' },
          { label: 'Cloud price model', helpin: 'Per workspace', competitor: 'Per agent: $19, $39 or $99 a month' },
          { label: 'Cloud AI', helpin: HELPIN.ai, competitor: 'Captain credits, then $20 per 1,000' },
          { label: 'Free trial', helpin: HELPIN.trial, competitor: '15 days' },
        ],
      },
      {
        group: 'Support',
        rows: [
          { label: 'Channels', helpin: HELPIN.channels, competitor: 'Chat, email, WhatsApp, social, Telegram, LINE, SMS, voice' },
          { label: 'Help center', helpin: HELPIN.helpCenter, competitor: 'Built in, with search and locales' },
          { label: 'Widget SDKs', helpin: HELPIN.sdks, competitor: 'Web, React Native, Flutter' },
          { label: 'SLA policies and SSO', helpin: false, competitor: 'Enterprise plan' },
        ],
      },
      {
        group: 'Beyond support',
        rows: [
          { label: 'Project management', helpin: HELPIN.projects, competitor: 'Through the Linear integration' },
          { label: 'CRM with deals', helpin: HELPIN.crm, competitor: 'Contacts, companies and segments' },
          { label: 'Meeting notes', helpin: HELPIN.meetings, competitor: false },
          { label: 'Coding agents', helpin: 'Helpin Cloud; coming to Community in 0.2', competitor: false },
          { label: 'MCP server', helpin: HELPIN.mcp, competitor: 'Community-built only' },
        ],
      },
    ],
    differencesTitle: 'Where the two products take different paths.',
    differences: [
      {
        title: 'What is open',
        body: 'Chatwoot’s core is MIT-licensed, while features such as the Captain AI agent, SSO and SLAs sit in a paid enterprise edition, even when self-hosted. Every Helpin product feature is open source under AGPL-3.0; only Cloud billing code is separate.',
      },
      {
        title: 'After the conversation',
        body: 'Chatwoot links issues to Linear. Helpin includes roadmaps, sprints and objectives, so the request, the task and the follow-up share one history. Coding agents can open a pull request for review on Helpin Cloud, and are coming to the Community edition in 0.2.',
      },
      {
        title: 'Customer relationships',
        body: 'Chatwoot has contacts, companies and segments. Helpin adds deals and pipelines, plus meeting notes from Meet, Zoom, Teams and Webex, attached to the same account.',
      },
      {
        title: 'Maturity',
        body: 'Chatwoot has years of releases and a large community. Helpin’s Community edition is a 0.1 beta, so expect a younger project that is moving quickly.',
      },
    ],
    strengths: [
      { title: 'Channels.', body: 'WhatsApp, Facebook, Instagram, TikTok, Telegram, LINE, SMS and voice. Helpin supports web chat and email.' },
      { title: 'Maturity and community.', body: 'Tens of thousands of GitHub stars and roughly monthly releases.' },
      { title: 'Deployment options.', body: 'Official Docker, Kubernetes Helm charts, a Linux installer and cloud marketplace images.' },
      { title: 'Mobile.', body: 'An agent app for iOS and Android, and mobile widget SDKs.' },
    ],
    cost: {
      title: 'What a team of eight might pay.',
      lede: 'Self-hosted, both core products are free. The difference is the AI agent.',
      helpin: { amount: '$0 + your AI provider', detail: 'The Community edition includes the support and workspace AI agents. Connect OpenAI, Anthropic, OpenRouter or a compatible endpoint and pay the provider directly.' },
      competitor: { amount: '$152 a month + your AI provider', detail: 'Self-hosted Captain AI needs the Premium plan at $19 an agent, and your own OpenAI-compatible key.' },
      note: 'On Cloud, Helpin Growth is $239 a month billed annually for the whole workspace. Chatwoot Business is $39 an agent, or $312 a month for eight.',
    },
    switching: {
      title: 'Moving from Chatwoot.',
      body: [
        'There is no Chatwoot importer yet. Both products have APIs, and help articles can be recreated in Helpin Knowledge.',
        SWITCH_NOTE,
      ],
    },
    faqs: [
      ['Are Helpin and Chatwoot both open source?', 'Yes. Chatwoot’s core is MIT-licensed with a separate paid enterprise edition. Helpin’s product features are open source under AGPL-3.0.'],
      ['Which is better for self-hosting?', 'Chatwoot is more mature, with more deployment options. Helpin includes AI agents, projects and CRM in the free Community edition, which is currently a 0.1 beta.'],
      ['Can I use my own AI provider?', 'Yes, with both. Self-hosted Helpin connects to OpenAI, Anthropic, OpenRouter or an OpenAI-compatible endpoint. Chatwoot’s Captain supports your own OpenAI-compatible key on a paid plan.'],
      ['Does Helpin support WhatsApp?', 'Not today. Helpin supports web chat and email. If WhatsApp or social channels are essential, Chatwoot covers them.'],
    ],
    closing: { title: 'Open source, from the conversation to the release.', description: 'Self-host the Community edition for free, or start a 14-day trial of Helpin Cloud with no card.' },
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
    category: 'Project management',
    cardLine: 'A fast issue tracker that connects to your support tool, compared with projects and support in one product.',
    checked: CHECKED,
    seo: {
      title: 'Helpin vs Linear: project management with customer context',
      description: 'Compare Helpin Projects and Linear: roadmaps, sprints, coding agents and pricing, and what changes when support, CRM and meetings live alongside the work.',
    },
    hero: {
      title: 'Helpin vs Linear: plan the work with the customer attached.',
      lede: 'Linear is a fast, focused issue tracker that connects to your support tool. Helpin includes projects and support in one product, along with CRM, meetings and docs, so the request, the task and the follow-up share one history.',
    },
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
    tableLede: 'Checkmarks mean the capability is available. Where a plan matters, it is noted.',
    table: [
      {
        group: 'Pricing',
        rows: [
          { label: 'Price model', helpin: 'Per workspace', competitor: 'Per user: Basic $10, Business $16 a month billed annually' },
          { label: 'Free plan', helpin: 'Self-hosted Community edition', competitor: 'Unlimited members, 2 teams, 250 issues' },
          { label: 'AI coding', helpin: 'From the AI allowance included in every Cloud plan', competitor: 'Prepaid AI credits for coding sessions' },
          { label: 'Open source and self-hosting', helpin: HELPIN.selfHost, competitor: false },
        ],
      },
      {
        group: 'Planning',
        rows: [
          { label: 'Roadmaps and sprints', helpin: true, competitor: 'Initiatives, projects and cycles' },
          { label: 'Triage', helpin: true, competitor: true },
          { label: 'GitHub and GitLab', helpin: 'PR and MR linking', competitor: 'PR and MR linking' },
          { label: 'Coding agents', helpin: HELPIN.coding, competitor: 'Linear Agent plus Cursor, Codex, Copilot and others' },
          { label: 'Native mobile apps', helpin: false, competitor: 'iOS and Android' },
        ],
      },
      {
        group: 'Customers',
        rows: [
          { label: 'Support inbox and live chat', helpin: HELPIN.channels, competitor: 'Integrations with Intercom and Zendesk on Business' },
          { label: 'Help center', helpin: HELPIN.helpCenter, competitor: false },
          { label: 'CRM with deals', helpin: HELPIN.crm, competitor: false },
          { label: 'Meeting notes', helpin: HELPIN.meetings, competitor: 'Gong transcripts on Enterprise' },
          { label: 'MCP server', helpin: HELPIN.mcp, competitor: 'Hosted' },
        ],
      },
    ],
    differencesTitle: 'Where the two products take different paths.',
    differences: [
      {
        title: 'Where customer requests come from',
        body: 'Linear links requests from tools like Intercom and Zendesk, which needs its Business plan. In Helpin the support inbox, help center and CRM are part of the product, so the conversation is already attached to the task.',
      },
      {
        title: 'What the coding agent sees',
        body: 'Both can hand an issue to a coding agent. Helpin’s agents also see the customer conversation, earlier workarounds and account context behind the task, and open a pull request for your team to review.',
      },
      {
        title: 'Closing the loop',
        body: 'When work ships in Linear, the update goes back through your support tool. In Helpin, the same history holds the release and the original conversation, so the team can follow up with the customer from one place.',
      },
      {
        title: 'Pricing and hosting',
        body: 'Linear charges per user and is hosted only. Helpin charges per workspace with unlimited teammates, and is open source, so you can self-host it.',
      },
    ],
    strengths: [
      { title: 'Speed and design.', body: 'A fast, keyboard-first tracker that engineering teams consistently praise.' },
      { title: 'Adoption.', body: 'Used by more than 40,000 companies, with a large community of templates and practices.' },
      { title: 'Agent ecosystem.', body: 'Assign issues to Linear Agent or to third-party agents such as Cursor, Codex, Copilot and Devin.' },
      { title: 'Mobile and imports.', body: 'Native iOS and Android apps, and importers for Jira, GitHub Issues, Asana and Shortcut.' },
    ],
    cost: {
      title: 'What a team of twenty might pay.',
      lede: 'List prices for twenty people across engineering, product and support, billed annually.',
      helpin: { amount: '$239 a month', detail: 'Growth plan for the whole workspace: projects, support, CRM, meetings and docs, with $239 of AI usage included each month.' },
      competitor: { amount: '$320 a month + support tool', detail: 'Business plan at $16 a user, which includes the Intercom and Zendesk integrations. Your support tool is priced separately, and coding sessions use prepaid AI credits.' },
      note: 'Linear’s Basic plan is $10 a user, but customer-request integrations with support tools need Business.',
    },
    switching: {
      title: 'Moving from Linear.',
      body: [
        'There is no Linear importer yet. Linear exports issues to CSV and through its API. Helpin’s agents can also connect to Linear through MCP, so you can start with Helpin for support while engineering stays in Linear.',
        'Helpin can import projects from Shortcut today.',
      ],
    },
    faqs: [
      ['Is Helpin a Linear alternative?', 'For teams that want projects and customer support in one product, yes. If you want a dedicated issue tracker and already use a separate support tool, Linear is a strong choice.'],
      ['Can I use Helpin and Linear together?', 'Yes. Helpin’s agents can use Linear’s tools through MCP, so support can run in Helpin while engineering stays in Linear.'],
      ['Does Linear have customer support features?', 'Linear has no customer-facing inbox, live chat or help center. It links customer requests from support tools such as Intercom and Zendesk.'],
      ['Do both have coding agents?', 'Yes. Linear Agent can write code in cloud sessions, and third-party agents can be assigned issues. Helpin’s coding agents work from the task and the customer conversation behind it, and open pull requests for review.'],
    ],
    closing: { title: 'Plan the work with the customer in view.', description: 'Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free.' },
    sources: [
      { label: 'Linear pricing', url: 'https://linear.app/pricing' },
      { label: 'Linear customer requests', url: 'https://linear.app/docs/customer-requests' },
      { label: 'Linear AI credits', url: 'https://linear.app/docs/ai-credits' },
      { label: 'Linear coding sessions', url: 'https://linear.app/changelog/2026-06-11-coding-sessions' },
      { label: 'Agents in Linear', url: 'https://linear.app/docs/agents-in-linear' },
      { label: 'Linear imports', url: 'https://linear.app/docs/import-issues' },
      { label: 'Linear MCP server', url: 'https://linear.app/docs/mcp' },
      { label: 'Linear mobile apps', url: 'https://linear.app/mobile' },
    ],
  },
];
