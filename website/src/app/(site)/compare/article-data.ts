// Long-form copy for each comparison article. Competitor facts come from the research
// recorded in compare-data.ts (`sources`, `checked`); Helpin facts must match the product
// and /pricing. Shared Helpin paragraphs keep every page describing Helpin the same way.
import type { ProductPreviewName } from '../_components/product-previews/ProductPreview';

export type ArticleFeature = {
  id: string;
  title: string;
  competitor: string;
  helpin: string;
  verdict: string;
  preview?: { product: ProductPreviewName; label: string };
};

export type Article = {
  intro: string[];
  difference: { lead: string; text: string }[];
  features: ArticleFeature[];
  pricing: string;
  hosting: string;
  onboarding: string;
};

const HELPIN = {
  inbox: 'Helpin handles web chat and email in one shared inbox, with team inboxes, tags, saved replies, internal notes, business hours and CSAT surveys. AI conversation routing and round-robin assignment are part of the Growth plan. It doesn’t offer phone, WhatsApp or social channels today.',
  ai: 'Helpin’s AI agents answer from your docs and the customer’s history, and hand off to your team with what they found. They can also go further: plan the work, open a pull request for review and prepare the follow-up. You choose each agent’s tools and which actions need approval, and usage comes from an allowance included in every Cloud plan.',
  knowledge: 'Helpin Knowledge publishes help articles, product guides and interactive API reference docs on a Helpin address, your own domain, or a path like /docs through a reverse proxy. Agents draft updates from unanswered questions and shipped changes, and nothing goes live until your team publishes it.',
  work: 'Helpin includes projects with roadmaps, sprints, objectives and epics. A conversation can become a task with the customer’s history attached, a coding agent can open a GitHub pull request or GitLab merge request for review, and the team can follow up in the original conversation when the fix ships.',
  crm: 'Helpin includes a CRM with contacts, companies, deals and pipelines, and a meeting notetaker for Google Meet, Zoom, Microsoft Teams and Webex. Decisions and action items from calls stay on the same customer record as the conversations and tasks.',
  developers: 'Helpin offers web SDKs for JavaScript, React, Next.js and Vue, a Helpin MCP server (in beta, hosted or self-hosted), connections to external MCP servers for agents, and GitHub and GitLab integrations. It doesn’t have native mobile SDKs or a large app marketplace yet.',
  pricing: 'Helpin charges one price per workspace, with unlimited teammates. Starter is $79 and Growth $239 a month billed annually ($99 and $299 billed monthly), and each includes an AI usage allowance. The self-hosted Community edition is free.',
  hosting: 'Helpin is open source under AGPL-3.0. Use Helpin Cloud, or run the Community edition (0.2 beta) on your own infrastructure with Docker Compose and your own AI provider. Nothing leaves your servers unless you connect it.',
  onboarding: 'Helpin’s trial runs for 14 days on the Growth plan, with no card. Our team will help you plan the move: what to set up first, how to run both tools side by side, and when to cut over.',
};

const PREVIEW = {
  inbox: { product: 'inbox', label: 'Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view.' },
  agents: { product: 'agents', label: 'Helpin’s specialist agents, each with its own tools and approval rules.' },
  knowledge: { product: 'knowledge', label: 'Helpin Knowledge: help articles and API reference docs on your own domain.' },
  projects: { product: 'projects', label: 'Helpin Projects: the task, the customer conversation behind it and the agent’s progress.' },
  crm: { product: 'crm', label: 'Helpin CRM: deals with the conversations, meetings and work behind them.' },
} as const satisfies Record<string, { product: ProductPreviewName; label: string }>;

const guide = (name: string) => `This guide compares Helpin and ${name} feature by feature, with list prices checked in September 2026, so you can decide which fits the way your team works.`;

export const ARTICLES: Record<string, Article> = {
  intercom: {
    intro: [
      'Intercom and Helpin both help support teams answer customers with AI. They differ in what happens after the answer, in how AI is billed, and in where the product can run.',
      'Intercom is a mature, support-first platform built around its Fin AI agent, with a broad set of channels and a large app marketplace. In May 2026 the company renamed itself Fin, and Salesforce completed its acquisition in September 2026. The helpdesk product is still called Intercom.',
      'Helpin is a newer, open-source platform that puts support, projects, CRM, meetings and docs on one customer history, with AI agents that can take a question all the way to a shipped fix.',
      guide('Intercom'),
    ],
    difference: [
      { lead: 'Intercom is built around the conversation:', text: 'resolving it quickly across many channels, with Fin handling a growing share on its own.' },
      { lead: 'Helpin is built around the customer history:', text: 'the conversation, the task it becomes, the pull request that fixes it and the follow-up when it ships.' },
    ],
    features: [
      {
        id: 'inbox', title: 'Inbox and channels',
        competitor: 'Intercom’s shared inbox covers Messenger chat and email on every plan, with phone, WhatsApp, SMS, social channels, Slack, Discord and Microsoft Teams available, several of them billed by usage. Workflows, multiple team inboxes and round-robin assignment arrive on the Advanced plan.',
        helpin: HELPIN.inbox,
        verdict: 'If you need phone or messaging apps in the same inbox, Intercom covers far more channels. If chat and email are your main channels, both handle the everyday work well.',
        preview: PREVIEW.inbox,
      },
      {
        id: 'ai-agents', title: 'AI agents',
        competitor: 'Fin is Intercom’s AI agent. It answers from your content and data, hands conversations to your team, and can run multi-step procedures. It’s billed at $0.99 per outcome on top of seats. Copilot helps teammates draft replies, with 10 free conversations per teammate each month before a paid upgrade.',
        helpin: HELPIN.ai,
        verdict: 'Fin is a proven, support-focused agent. Helpin’s agents work across support and the product work behind it, with usage included in the plan price.',
        preview: PREVIEW.agents,
      },
      {
        id: 'knowledge', title: 'Help center and knowledge',
        competitor: 'Intercom includes a public help center on every plan, with multilingual and private help centers on Advanced and multibrand help centers on Expert. Fin uses your articles and other content to answer.',
        helpin: HELPIN.knowledge,
        verdict: 'Both publish a help center that feeds their AI. Helpin adds API reference docs and agent-drafted updates your team reviews.',
      },
      {
        id: 'question-to-fix', title: 'From question to fix',
        competitor: 'Intercom resolves the conversation and hands engineering work to other tools, such as its Jira integration. The conversation lives in Intercom and the task lives elsewhere, so context has to travel between them.',
        helpin: HELPIN.work,
        verdict: 'If support often hands off to engineering, keeping both on one history is Helpin’s biggest difference.',
        preview: PREVIEW.projects,
      },
      {
        id: 'crm', title: 'Customer records, CRM and meetings',
        competitor: 'Intercom keeps contacts, companies and leads, and connects to CRMs such as Salesforce and HubSpot for deals. It doesn’t record meetings.',
        helpin: HELPIN.crm,
        verdict: 'Teams that already run a separate CRM may be happy with Intercom’s integrations. Teams that want one record per customer get it built in with Helpin.',
        preview: PREVIEW.crm,
      },
      {
        id: 'developers', title: 'Developers and integrations',
        competitor: 'Intercom offers a REST API, iOS, Android and React Native SDKs, more than 450 apps and integrations, and a hosted MCP server for AI tools.',
        helpin: HELPIN.developers,
        verdict: 'Intercom’s ecosystem is far larger today. Helpin focuses on connecting your product, your code and your AI tools.',
      },
    ],
    pricing: 'Intercom charges per seat: $29, $85 or $132 a month billed annually for Essential, Advanced and Expert. Fin adds $0.99 per outcome, and channels such as phone, SMS and WhatsApp, and add-ons such as Copilot and Proactive Support, are billed separately.',
    hosting: 'Intercom is a hosted service, and there’s no way to run it on your own infrastructure.',
    onboarding: 'Intercom offers a 14-day trial with no card. It has no one-click importer: moving historical data into Intercom is a scripted, API-based migration, and conversations can be exported as CSV, through the API or to cloud storage.',
  },
  zendesk: {
    intro: [
      'Zendesk and Helpin both run customer support with AI agents. Zendesk is an established help desk for large, multichannel service teams. Helpin is a newer, open-source platform that connects support to the work behind it.',
      'Zendesk offers ticketing, messaging, a native contact center and deep admin controls, with AI agents billed on verified resolutions. It has announced that Zendesk Sell, its sales CRM, will be retired in August 2027.',
      'Helpin keeps support, projects, CRM, meetings and docs on one customer history, with one price per workspace and an open-source edition you can run yourself.',
      guide('Zendesk'),
    ],
    difference: [
      { lead: 'Zendesk is built for service operations:', text: 'routing high volumes of tickets across channels, with the controls large teams need.' },
      { lead: 'Helpin is built for the work after the ticket:', text: 'the task, the fix, the account and the follow-up, on one customer history.' },
    ],
    features: [
      {
        id: 'inbox', title: 'Ticketing and channels',
        competitor: 'Zendesk covers email, messaging and live chat, social messaging, and voice with IVR through its contact center, with skills-based routing on Suite Professional. Ticketing, macros, automations and SLA policies are deep and mature.',
        helpin: HELPIN.inbox,
        verdict: 'For large, multichannel teams, Zendesk’s routing and controls go further. For chat and email support, both cover the everyday work of a support team.',
        preview: PREVIEW.inbox,
      },
      {
        id: 'ai-agents', title: 'AI agents',
        competitor: 'Zendesk’s AI agents are included in every Suite plan and billed per verified resolution beyond a small included allowance; escalated and contained conversations aren’t billed, and Zendesk doesn’t publish the price per resolution. Copilot, which assists your agents, is a $50 per agent add-on on most plans.',
        helpin: HELPIN.ai,
        verdict: 'Zendesk’s AI sits inside a mature ticketing system. Helpin’s agents also plan and code the fix, with usage included in the plan price.',
        preview: PREVIEW.agents,
      },
      {
        id: 'knowledge', title: 'Help center and knowledge',
        competitor: 'Zendesk includes a knowledge base and help center on Suite plans, which its AI agents draw on to answer customers.',
        helpin: HELPIN.knowledge,
        verdict: 'Both publish customer help content for people and AI. Helpin adds API reference docs and agent-drafted updates.',
      },
      {
        id: 'question-to-fix', title: 'From ticket to fix',
        competitor: 'Zendesk manages the ticket and connects to tools like Jira for engineering work, so the fix and its status live in another system.',
        helpin: HELPIN.work,
        verdict: 'If tickets often become engineering work, keeping both on one history is Helpin’s biggest difference.',
        preview: PREVIEW.projects,
      },
      {
        id: 'crm', title: 'CRM and meetings',
        competitor: 'Zendesk Sell is Zendesk’s sales CRM, but Zendesk plans to retire it on August 31, 2027 and recommends Pipedrive as a migration partner. Zendesk doesn’t record meetings.',
        helpin: HELPIN.crm,
        verdict: 'If you rely on Zendesk Sell, you’ll need a new CRM by 2027. Helpin includes one, on the same customer history as support.',
        preview: PREVIEW.crm,
      },
      {
        id: 'developers', title: 'Developers and integrations',
        competitor: 'Zendesk offers REST APIs, iOS and Android SDKs and more than 1,800 marketplace apps. It’s moving API access from tokens to OAuth, and has announced an MCP server, with an MCP client in early access.',
        helpin: HELPIN.developers,
        verdict: 'Zendesk’s marketplace is one of the largest in support software. Helpin focuses on your product, your code and your AI tools.',
      },
    ],
    pricing: 'Zendesk charges per agent: Suite Team is $55 and Suite Professional $115 a month billed annually, with Enterprise priced on request. AI agent resolutions beyond the included allowance, and Copilot at $50 per agent, are billed separately.',
    hosting: 'Zendesk is a hosted service, and there’s no way to run it on your own infrastructure.',
    onboarding: 'Zendesk offers a 14-day trial with no card, running on Suite Professional. Ticket and user exports are off by default and must be enabled by Zendesk on request, and full JSON exports need higher plans.',
  },
  'help-scout': {
    intro: [
      'Help Scout and Helpin both give small and growing teams a shared inbox with AI. Help Scout keeps support simple. Helpin connects support to projects, CRM, meetings and docs.',
      'Help Scout is a well-loved, email-like inbox with a Docs knowledge base and a free plan for up to five users. It returned to per-user pricing after a period of contact-based billing, and charges $0.75 per AI resolution.',
      'Helpin keeps support on one customer history with the work that follows, charges one price per workspace, and can run on your own servers.',
      guide('Help Scout'),
    ],
    difference: [
      { lead: 'Help Scout is built around a calm inbox:', text: 'fast, simple conversations with customers across email, chat and messaging.' },
      { lead: 'Helpin is built around what happens next:', text: 'turning requests into tracked work, deals and follow-ups on the same history.' },
    ],
    features: [
      {
        id: 'inbox', title: 'Inbox and channels',
        competitor: 'Help Scout’s inbox covers email, live chat through Beacon, WhatsApp, Instagram, Messenger and SMS, with phone through integrations such as Aircall. Workflows, SLA policies and inbox limits scale with the plan: Standard includes two inboxes and Plus five.',
        helpin: HELPIN.inbox,
        verdict: 'Help Scout covers more messaging channels. Both keep everyday email and chat support simple.',
        preview: PREVIEW.inbox,
      },
      {
        id: 'ai-agents', title: 'AI answers and agents',
        competitor: 'Help Scout’s AI Answers resolves questions from your Docs at $0.75 per resolution, with spending caps and a discount for prepaying. AI Assist is included on paid plans, and AI Drafts and Summarize come with Plus and Pro.',
        helpin: HELPIN.ai,
        verdict: 'Help Scout’s AI focuses on answering. Helpin’s agents also plan the fix and follow up, with usage included in the price.',
        preview: PREVIEW.agents,
      },
      {
        id: 'knowledge', title: 'Help center and knowledge',
        competitor: 'Help Scout Docs publishes knowledge base sites: two on Standard and three on Plus, with more available as an add-on. AI Answers draws on them.',
        helpin: HELPIN.knowledge,
        verdict: 'Both publish help centers that feed AI answers. Helpin can import your Help Scout Docs today.',
      },
      {
        id: 'question-to-fix', title: 'From question to fix',
        competitor: 'Help Scout connects to Jira on the Plus plan and above, so engineering work lives in another tool.',
        helpin: HELPIN.work,
        verdict: 'If your conversations often turn into product work, Helpin keeps the request and the fix together.',
        preview: PREVIEW.projects,
      },
      {
        id: 'crm', title: 'Customer records, CRM and meetings',
        competitor: 'Help Scout keeps customer properties, company profiles and, on higher plans, custom fields, and integrates with HubSpot and Salesforce on Plus. It doesn’t include a deals pipeline or meeting notes.',
        helpin: HELPIN.crm,
        verdict: 'If you need deals and meeting notes next to support, Helpin has them built in.',
        preview: PREVIEW.crm,
      },
      {
        id: 'developers', title: 'Developers and integrations',
        competitor: 'Help Scout offers Inbox, Docs and Chat APIs, webhooks, Beacon SDKs for iOS and Android, more than 100 integrations, and a read-only MCP server launched in July 2026.',
        helpin: HELPIN.developers,
        verdict: 'Help Scout has mobile SDKs and a wider integration list. Helpin’s MCP connections let agents act, not just read.',
      },
    ],
    pricing: 'Help Scout charges per user: Standard is $25 and Plus $45 a month billed annually, with Pro from ten users. A free plan covers up to five users and 100 contacts a month, and AI Answers adds $0.75 per resolution.',
    hosting: 'Help Scout is a hosted service, and there’s no way to run it on your own infrastructure.',
    onboarding: 'Help Scout offers a 15-day trial with no card and a 30-day money-back guarantee on the first payment. Its built-in importer brings conversations, customers and tags from more than 30 tools. Helpin imports Help Scout Docs today; conversation history isn’t imported yet.',
  },
  chatwoot: {
    intro: [
      'Chatwoot and Helpin are both open-source customer support platforms you can run on your own servers. They differ in license, in what the free edition includes, and in how far each goes beyond the inbox.',
      'Chatwoot is a mature, widely used support inbox with a long list of channels, from WhatsApp to LINE and voice. Its core is MIT-licensed, and features such as the Captain AI agent, SSO and SLAs need a paid enterprise edition, even when self-hosted.',
      'Helpin is a newer platform under AGPL-3.0 that connects support to projects, CRM, meetings and docs. Its Community edition, currently a 0.2 beta, includes every AI agent at no cost, coding agents too.',
      guide('Chatwoot'),
    ],
    difference: [
      { lead: 'Chatwoot is built around channels:', text: 'one inbox for every way customers reach you, backed by a large open-source community.' },
      { lead: 'Helpin is built around the customer history:', text: 'support, projects, CRM and meetings in one place, with agents working across them.' },
    ],
    features: [
      {
        id: 'inbox', title: 'Inbox and channels',
        competitor: 'Chatwoot’s inbox covers website chat, email, WhatsApp, Facebook, Instagram, TikTok, Telegram, LINE and SMS, with voice calls in beta. On Cloud, automation rules, teams and voice need the Business plan.',
        helpin: HELPIN.inbox,
        verdict: 'Chatwoot covers far more channels. If chat and email are enough, both handle the inbox well.',
        preview: PREVIEW.inbox,
      },
      {
        id: 'ai-agents', title: 'AI agents',
        competitor: 'Captain is Chatwoot’s AI agent and copilot. On Cloud it runs on monthly credits, with more at $20 per 1,000. Self-hosted, it needs a paid plan and your own OpenAI-compatible key.',
        helpin: `${HELPIN.ai} Self-hosted, every agent, coding agents included, is part of the free Community edition.`,
        verdict: 'Both let you bring your own model when self-hosting. Helpin includes its agents in the free edition, and they go beyond the inbox.',
        preview: PREVIEW.agents,
      },
      {
        id: 'knowledge', title: 'Help center and knowledge',
        competitor: 'Chatwoot includes a help center with a documentation-style layout, search and per-locale branding, which Captain can use to answer.',
        helpin: HELPIN.knowledge,
        verdict: 'Both include a help center. Helpin adds API reference docs and agent-drafted updates.',
      },
      {
        id: 'question-to-fix', title: 'From conversation to fix',
        competitor: 'Chatwoot links conversations to issues through its Linear integration, so planning and engineering happen in another tool.',
        helpin: HELPIN.work,
        verdict: 'If conversations turn into product work, Helpin keeps the request, the task and the follow-up together.',
        preview: PREVIEW.projects,
      },
      {
        id: 'crm', title: 'Customer records, CRM and meetings',
        competitor: 'Chatwoot keeps contacts, custom attributes, segments and companies. It doesn’t include a deals pipeline or meeting notes.',
        helpin: HELPIN.crm,
        verdict: 'If you need deals and meeting notes next to support, Helpin has them built in.',
        preview: PREVIEW.crm,
      },
      {
        id: 'developers', title: 'Developers and integrations',
        competitor: 'Chatwoot offers application, platform and client APIs, webhooks, a CLI, and React Native and Flutter SDKs. MCP servers for Chatwoot are community-built.',
        helpin: HELPIN.developers,
        verdict: 'Chatwoot has mobile SDKs and a mature API. Helpin ships its own MCP server and connects agents to external tools.',
      },
    ],
    pricing: 'Chatwoot Cloud charges per agent: $19, $39 or $99 a month, with Captain credits included and extra credits at $20 per 1,000. Self-hosted, the Community edition is free, and the Premium plan that adds Captain costs $19 per agent.',
    hosting: 'Chatwoot is open source and can be deployed with Docker, Kubernetes, a Linux installer or cloud marketplace images. Features in its enterprise edition need a paid subscription for production use.',
    onboarding: 'Chatwoot offers a 15-day Cloud trial and a free Hacker plan for up to two agents, and can import from Intercom and Freshdesk. There’s no Chatwoot importer in Helpin yet; both products have APIs if you need to move records yourself.',
  },
  linear: {
    intro: [
      'Linear and Helpin both help teams plan and ship software with AI agents. Linear is a fast, focused issue tracker. Helpin puts projects next to support, CRM, meetings and docs.',
      'Linear is used by more than 40,000 companies and is known for its speed and keyboard-first design. It links customer requests from support tools such as Intercom and Zendesk, and lets you hand issues to Linear Agent or to third-party coding agents.',
      'Helpin includes roadmaps, sprints and objectives alongside a support inbox and CRM, so the request, the task, the pull request and the follow-up share one customer history. It’s open source and charges per workspace.',
      guide('Linear'),
    ],
    difference: [
      { lead: 'Linear is built around the issue:', text: 'planning and shipping engineering work quickly, with customer context linked in from other tools.' },
      { lead: 'Helpin is built around the customer:', text: 'the request, the work it becomes and the follow-up, in one product.' },
    ],
    features: [
      {
        id: 'planning', title: 'Roadmaps, sprints and triage',
        competitor: 'Linear organizes work into issues, projects, cycles and initiatives, with a triage inbox, releases and diffs on every plan, and Insights dashboards on Business. It’s widely praised for its speed and design.',
        helpin: 'Helpin Projects covers roadmaps, sprints, objectives, epics, dependencies, estimates and triage, with velocity and sprint reports. Tasks can carry the customer requests and conversations behind them.',
        verdict: 'For a pure engineering tracker, Linear is fast and polished. Helpin’s planning is built to keep customer context on the work.',
        preview: PREVIEW.projects,
      },
      {
        id: 'coding-agents', title: 'Coding agents',
        competitor: 'Linear Agent can write code in cloud coding sessions using Claude Code or Codex, paid for with prepaid AI credits, and you can assign issues to third-party agents such as Cursor, Codex, GitHub Copilot and Devin.',
        helpin: 'Helpin’s coding agents work from the task and the customer conversation behind it, open a GitHub pull request or GitLab merge request, and wait for your team’s review. On Cloud, usage comes from the AI allowance in your plan; self-hosted, they’re part of the free Community edition and use your own AI provider.',
        verdict: 'Linear offers the widest choice of agents. Helpin’s agents start with more of the customer context.',
        preview: PREVIEW.agents,
      },
      {
        id: 'customer-requests', title: 'Customer requests and support',
        competitor: 'Linear has no customer-facing inbox, live chat or help center. It links customer requests from Slack on every plan, and from Intercom, Zendesk and Front on the Business plan.',
        helpin: 'Helpin includes the support inbox, chat widget and help center, so requests start in the same product as the task. When the work ships, the team can follow up in the original conversation.',
        verdict: 'If you’re happy with your support tool, Linear links to it well. If you want support and projects together, Helpin removes the hand-off.',
        preview: PREVIEW.inbox,
      },
      {
        id: 'crm', title: 'Customer records and meetings',
        competitor: 'Linear tracks customer requests and attributes, and can turn Gong call transcripts into requests on Enterprise. It has no CRM or meeting notes of its own.',
        helpin: HELPIN.crm,
        verdict: 'If account context matters to your roadmap, Helpin keeps deals and meetings next to the work.',
        preview: PREVIEW.crm,
      },
      {
        id: 'developers', title: 'Git, API and integrations',
        competitor: 'Linear connects to GitHub and GitLab for pull request linking, and offers a GraphQL API, webhooks, a hosted MCP server and native iOS and Android apps.',
        helpin: 'Helpin links GitHub pull requests and GitLab merge requests to tasks and updates delivery status when work merges. It offers a Helpin MCP server (in beta) and connections to external MCP servers, including Linear’s.',
        verdict: 'Both connect to your code. Helpin can also reach Linear through MCP, so the two can run side by side.',
      },
      {
        id: 'knowledge', title: 'Docs and knowledge',
        competitor: 'Linear includes documents for specs and project updates, but no public help center for customers.',
        helpin: HELPIN.knowledge,
        verdict: 'If you publish docs for customers, Helpin covers that in the same product as your roadmap.',
      },
    ],
    pricing: 'Linear charges per user: Basic is $10 and Business $16 a month billed annually, with a free plan for up to 250 issues. Coding sessions use prepaid AI credits, the integrations with support tools need Business, and your support tool is priced separately.',
    hosting: 'Linear is a hosted service, and there’s no way to run it on your own infrastructure.',
    onboarding: 'Linear imports from Jira, GitHub Issues, Asana and Shortcut, and exports issues as CSV or through its API. Helpin doesn’t import from Linear yet, but its agents can use Linear’s tools through MCP, so you can run both during a move.',
  },
};

export const HELPIN_ARTICLE = HELPIN;
