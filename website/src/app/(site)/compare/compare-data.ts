// Comparison page content. One entry per competitor; the /compare/[slug] template renders it.
// Competitor facts come from their public pages on the `checked` date (see `sources`).
// Helpin facts must match the shipped product and /pricing. Re-check at least quarterly,
// then update each checked date (it also sets the year in each page title).
import type { PageSeo } from '../../../lib/metadata.ts';

export type Cell = boolean | string;
export type Status = 'yes' | 'partial' | 'no';
type FAQ = readonly [question: string, answer: string];

// Icons are resolved in ComparePage so this file stays plain data.
export type IconKey =
  | 'workflow' | 'billing' | 'team' | 'hosting' | 'crm' | 'open' | 'maturity' | 'requests' | 'agents' | 'loop'
  | 'channels' | 'ecosystem' | 'enterprise' | 'reporting' | 'messaging' | 'simplicity' | 'import' | 'mobile'
  | 'community' | 'deploy' | 'speed' | 'models' | 'docs';

type StepStatus = 'Available now' | 'Beta' | 'Coming soon' | 'Not yet';

type TableRow = { label: string; helpin: Cell; competitor: Cell; helpinStatus?: Status; competitorStatus?: Status };

export type CalculatorPlan = {
  name: string;
  annual: number;
  monthly?: number;
  minSeats?: number;
  /** Total yearly prices for products billed in user bands, within the slider range. */
  annualTiers?: { maxSeats: number; price: number }[];
};

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
  hero: { lede: string };
  /** Three short rows for the hero's comparison cards: where the products differ most. */
  glance: { label: string; competitor: string; helpin: string }[];
  summary: { title: string; lede: string; competitor: string[]; helpin: string[] };
  /** Common reasons teams look beyond the competitor, each with Helpin's approach. */
  /** Omit competitorLane when the competitor's side can't be stated from verified sources. */
  reasons: { title: string; body: string; icon: IconKey; competitorLane?: string[]; helpinLane: string[] }[];
  tableLede: string;
  table: { group: string; rows: TableRow[] }[];
  strengths: { title: string; body: string; icon: IconKey }[];
  calculator?: Calculator;
  switching: {
    title: string;
    lede: string;
    take: string[];
    setUp: string[];
    steps: { title: string; body: string; status: StepStatus }[];
  };
  faqs: FAQ[];
  closing: { title: string; description: string };
  /** Defaults to a comparison film; media can point to an existing product overview. */
  video: { seconds: number; summary: string; media?: { title: string; src: string; poster: string; published: string } };
  /** First-party sources shown in the page's source list. */
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
    imagePath: `/og/helpin-compare-${competitor.slug}-green-v5.png`,
    imageAlt: `Helpin vs ${competitor.name}`,
  };
}

const VIDEO_PUBLISHED = '2026-09-26';

/** Files and metadata for a competitor's comparison video. Bump the -v suffix when a video is re-cut. */
export function compareVideo(competitor: Competitor) {
  const base = `/new/compare/helpin-vs-${competitor.slug}`;
  return {
    ...competitor.video,
    title: `Helpin vs ${competitor.name} in ${competitor.video.seconds} seconds`,
    src: `${base}-1080p-v1.mp4`,
    poster: `${base}-poster-1920-v1.webp`,
    published: VIDEO_PUBLISHED,
    ...competitor.video.media,
  };
}

/** The status a cell shows: explicit when given, otherwise implied by a yes/no value. */
export function cellStatus(value: Cell, status?: Status): Status | undefined {
  if (status) return status;
  if (value === true) return 'yes';
  if (value === false) return 'no';
  return undefined;
}

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

export const COMPETITORS: Competitor[] = [
  {
    slug: 'intercom',
    name: 'Intercom',
    group: 'Customer support',
    category: 'AI customer support',
    cardLine: "AI support that connects to code, docs and sales, with no seat or resolution fees.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Intercom Alternative: Helpin vs Intercom",
      "description": "Compare Helpin vs Intercom: AI agents for support, code and docs, all your customer context in one fast workspace, and no per-seat or per-resolution fees."
    },
    hero: {
      "lede": "Answer customers, fix the problems they report and keep the guides current. Helpin’s AI agents work across support, development and sales in one fast workspace, with the customer’s history available throughout."
    },
    glance: [
      {
        "label": "Pricing",
        "competitor": "Per seat, plus $0.99 per AI outcome",
        "helpin": "Per workspace, unlimited teammates, AI allowance"
      },
      {
        "label": "Beyond support",
        "competitor": "Projects and CRM through integrations",
        "helpin": "Projects, CRM and meetings built in"
      },
      {
        "label": "Hosting",
        "competitor": "Hosted by Intercom",
        "helpin": "Open source: Cloud or your servers"
      }
    ],
    summary: {
      "title": "Choose the workflow you want after the first reply.",
      "lede": "Helpin brings the conversation, the fix and the customer follow-up together. Check any channel or mobile requirements before you move.",
      "competitor": [
        "Your support operation depends on phone, WhatsApp or social channels.",
        "You require Intercom’s native mobile SDKs, product tours or SAML sign-in."
      ],
      "helpin": [
        "You want agents to investigate, code, review and update docs from the same customer context.",
        "You want fast chat and email support with automatic follow-ups and human handoff.",
        "You want your whole team involved without per-seat or per-resolution fees.",
        "You want built-in CRM and meetings, with the choice to self-host."
      ]
    },
    reasons: [
      {
        "title": "No per-seat or per-resolution fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan. There is no separate charge for each resolved conversation.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the work behind the reply.",
        "icon": "loop",
        "body": "A bug report can become a task with the customer’s conversation attached. Scribe plans, Forge writes and tests the code, and Lens reviews it. After the release is confirmed, configured automation can follow up with affected customers under your approval rules.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Build agents around the way you work.",
        "icon": "agents",
        "body": "Describe a job in plain language to build a custom agent or flow. Choose its tools, instructions, skills and approval rules. Agents can use connected services for account details, code and logs, so they can investigate before taking the next step.",
        "helpinLane": [
          "Your instructions",
          "Context and tools",
          "Action"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Pricing",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace",
            "competitor": "Per seat: $29, $85 or $132 a month billed annually"
          },
          {
            "label": "Teammates",
            "helpin": "Unlimited teammates on every plan",
            "competitor": "Each full seat is paid; Lite seats on higher plans"
          },
          {
            "label": "AI agent",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "Fin at $0.99 per outcome"
          },
          {
            "label": "Free trial",
            "helpin": "14 days, no card",
            "competitor": "14 days, no card"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Support",
        "rows": [
          {
            "label": "Channels",
            "helpin": "Web chat and email",
            "competitor": "Chat, email, phone, WhatsApp, SMS, social, Slack and more"
          },
          {
            "label": "AI answers with human handoff",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Public help center; multilingual on Advanced",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Widget SDKs",
            "helpin": "Web: JavaScript, React, Next.js, Vue",
            "competitor": "Web, iOS, Android, React Native"
          },
          {
            "label": "Support reporting",
            "helpin": "Knowledge-gap and project reports",
            "competitor": "Pre-built reports; custom reports on Advanced"
          },
          {
            "label": "Company single sign-on (SAML)",
            "helpin": false,
            "competitor": "Expert plan",
            "competitorStatus": "yes"
          },
          {
            "label": "Support response-time policies (SLAs)",
            "helpin": false,
            "competitor": "Expert plan",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Beyond support",
        "rows": [
          {
            "label": "Project management",
            "helpin": "Roadmaps, sprints, objectives, epics",
            "competitor": "Through integrations such as Jira",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Contacts and companies; deals through integrations",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Hosted",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "channels",
        "title": "Phone and social support.",
        "body": "Intercom provides phone, WhatsApp and social channels. Helpin’s shared inbox supports web chat and email."
      },
      {
        "icon": "mobile",
        "title": "Chat inside native mobile apps.",
        "body": "Intercom has iOS, Android and React Native SDKs. Helpin currently provides web SDKs."
      },
      {
        "icon": "enterprise",
        "title": "Company sign-in and response policies.",
        "body": "Intercom Expert includes SSO and SLA policies. Helpin’s agent permissions and approvals serve a different purpose."
      },
      {
        "icon": "messaging",
        "title": "In-app campaigns and tours.",
        "body": "Check Intercom’s add-ons if product tours, surveys and targeted outbound messages are central to your workflow."
      }
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
      [
        "Is Helpin a good Intercom alternative?",
        "Yes. Helpin combines fast chat and email support with agents that can investigate issues, work on code, update docs and follow up. Projects, CRM and meeting notes share the customer context. Cloud pricing is per workspace, with no per-resolution fee."
      ],
      [
        "Is Helpin cheaper than Intercom?",
        "Helpin avoids Intercom’s per-seat and per-outcome charges. The total depends on your team size, plan and AI usage. Use the calculator above with your own numbers, and consider the projects, CRM and meeting tools included in Helpin."
      ],
      [
        "Can Helpin import my Intercom data?",
        "An Intercom importer is coming soon. You can export conversations from Intercom today and start Helpin alongside it while you move over."
      ],
      [
        "Can we run Helpin alongside Intercom?",
        "Yes. Add the Helpin widget to a few pages or forward one support address, and keep Intercom for everything else while your team tries the workflow. Move the rest when you’re ready."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Solve the problem behind the conversation.",
      "description": "Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free."
    },
    video: { seconds: 39, summary: 'Intercom charges per seat plus $0.99 per AI outcome. Helpin charges one workspace price with AI usage included, and turns the answer into a task, a pull request and a follow-up.' },
    sources: [
      {
        "label": "Intercom plans explained",
        "url": "https://www.intercom.com/help/en/articles/9061614-fin-and-intercom-plans-explained"
      },
      {
        "label": "Intercom seats",
        "url": "https://www.intercom.com/help/en/articles/8205716-seats"
      },
      {
        "label": "Fin AI agent outcomes",
        "url": "https://www.intercom.com/help/en/articles/8205718-fin-ai-agent-outcomes"
      },
      {
        "label": "Intercom pricing FAQ",
        "url": "https://www.intercom.com/help/en/articles/8344190-pricing-faqs"
      },
      {
        "label": "Intercom free trial",
        "url": "https://www.intercom.com/help/en/articles/891-how-to-start-a-free-trial-of-fin-and-intercom"
      },
      {
        "label": "Intercom mobile SDK FAQ",
        "url": "https://www.intercom.com/help/en/articles/7669340-mobile-sdk-faqs"
      },
      {
        "label": "Intercom MCP server",
        "url": "https://developers.intercom.com/docs/guides/mcp"
      },
      {
        "label": "Intercom conversation export",
        "url": "https://www.intercom.com/help/en/articles/2046229-export-your-conversations-data"
      },
      {
        "label": "Fin procedures and connected data",
        "url": "https://www.intercom.com/help/en/articles/13459820-how-to-use-data-connectors-in-fin-procedures"
      },
      {
        "label": "External MCP connectors",
        "url": "https://www.intercom.com/help/en/articles/11461635-add-mcp-connectors-for-popular-apps-or-custom-mcps"
      }
    ],
  },
  {
    slug: 'zendesk',
    name: 'Zendesk',
    group: 'Customer support',
    category: 'Help desk',
    cardLine: "Take tickets through agent investigation, code review and follow-up in one workspace.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Zendesk Alternative: Helpin vs Zendesk",
      "description": "Compare Helpin vs Zendesk: AI agents for support, code and docs, all your customer context in one fast workspace, and no per-seat or per-resolution fees."
    },
    hero: {
      "lede": "Give AI agents the customer history and tools to work on the problem behind a ticket. Helpin connects support, code, docs and sales in a fast workspace, so your team can review the work and keep customers informed."
    },
    glance: [
      {
        "label": "Pricing",
        "competitor": "Per agent, plus AI per resolution",
        "helpin": "Per workspace, unlimited teammates, AI allowance"
      },
      {
        "label": "CRM",
        "competitor": "Zendesk Sell retires in 2027",
        "helpin": "Contacts, companies and deals built in"
      },
      {
        "label": "Hosting",
        "competitor": "Hosted by Zendesk",
        "helpin": "Open source: Cloud or your servers"
      }
    ],
    summary: {
      "title": "Bring support and delivery into the same workflow.",
      "lede": "Helpin is built for work that crosses team boundaries. Review the specific phone and service policies your support operation needs.",
      "competitor": [
        "You need a native phone queue, phone menus or skills-based call routing.",
        "You depend on Zendesk-specific apps, sandboxes or service response policies."
      ],
      "helpin": [
        "You want AI agents to carry a customer issue into a reviewed code change.",
        "You want support, projects, deals and meeting notes in one fast workspace.",
        "You want agents to keep guides current and follow up with customers.",
        "You want unlimited teammates, no per-resolution fee and the option to self-host."
      ]
    },
    reasons: [
      {
        "title": "No per-seat or per-resolution fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan. There is no separate charge for each resolved conversation.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the work behind the reply.",
        "icon": "loop",
        "body": "A bug report can become a task with the customer’s conversation attached. Scribe plans, Forge writes and tests the code, and Lens reviews it. After the release is confirmed, configured automation can follow up with affected customers under your approval rules.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Build agents around the way you work.",
        "icon": "agents",
        "body": "Describe a job in plain language to build a custom agent or flow. Choose its tools, instructions, skills and approval rules. Agents can use connected services for account details, code and logs, so they can investigate before taking the next step.",
        "helpinLane": [
          "Your instructions",
          "Context and tools",
          "Action"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Pricing",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace",
            "competitor": "Per agent: Suite Team $55, Professional $115 a month billed annually"
          },
          {
            "label": "Teammates",
            "helpin": "Unlimited teammates on every plan",
            "competitor": "Each agent is paid"
          },
          {
            "label": "AI agent",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "Billed per verified resolution, beyond a small included allowance"
          },
          {
            "label": "Free trial",
            "helpin": "14 days, no card",
            "competitor": "14 days, no card"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Support",
        "rows": [
          {
            "label": "Channels",
            "helpin": "Web chat and email",
            "competitor": "Email, messaging, chat, voice with IVR, social"
          },
          {
            "label": "AI answers with human handoff",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Knowledge base on Suite plans",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Widget SDKs",
            "helpin": "Web: JavaScript, React, Next.js, Vue",
            "competitor": "Web, iOS, Android"
          },
          {
            "label": "Support reporting",
            "helpin": "Knowledge-gap and project reports",
            "competitor": "Dashboards and analytics"
          },
          {
            "label": "Company single sign-on (SAML)",
            "helpin": false,
            "competitor": true,
            "competitorStatus": "yes"
          },
          {
            "label": "Support response-time policies (SLAs)",
            "helpin": false,
            "competitor": true,
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Beyond support",
        "rows": [
          {
            "label": "Project management",
            "helpin": "Roadmaps, sprints, objectives, epics",
            "competitor": "Through integrations such as Jira",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Zendesk Sell, retiring in August 2027",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "MCP connections for AI tools",
            "helpin": "MCP server and connections to external tools",
            "competitor": "Connect external tools to AI actions",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "channels",
        "title": "Phone queues and routing.",
        "body": "Zendesk provides a contact center with phone menus and skills-based routing. Helpin supports chat and email."
      },
      {
        "icon": "enterprise",
        "title": "SAML and service policies.",
        "body": "Check Zendesk’s company sign-in and SLA options if your organization requires them. Helpin does not currently offer those policies."
      },
      {
        "icon": "deploy",
        "title": "A separate testing environment.",
        "body": "Zendesk offers sandboxes on Enterprise. Check this requirement separately from the tool access and approval controls Helpin already provides."
      },
      {
        "icon": "mobile",
        "title": "Native mobile support.",
        "body": "Zendesk offers mobile SDKs and marketplace apps. Check the specific integration or mobile SDK your product needs."
      }
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
      [
        "Is Helpin a good Zendesk alternative?",
        "Yes. Helpin connects tickets to AI agents that can investigate and work on the underlying problem. Your team can review code, keep docs current and follow up from the same workspace, with projects, CRM and meetings built in."
      ],
      [
        "Is Helpin cheaper than Zendesk?",
        "Helpin charges per workspace with unlimited teammates and an included AI allowance. Zendesk charges per agent, with separate pricing for extra AI resolutions and Copilot. Compare your team’s numbers above, including any extra tools you would use beside Zendesk."
      ],
      [
        "What happens to Zendesk Sell?",
        "Zendesk has announced that Zendesk Sell will be retired on August 31, 2027. Helpin includes a CRM with contacts, companies and deals."
      ],
      [
        "Can Helpin import my Zendesk data?",
        "A Zendesk importer is coming soon. You can export data from Zendesk today and start Helpin alongside it while you move over."
      ],
      [
        "Can we run Helpin alongside Zendesk?",
        "Yes. Add the Helpin widget to a few pages or forward one support address, and keep Zendesk for everything else while your team tries the workflow. Move the rest when you’re ready."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Give your agents more than a ticket queue.",
      "description": "Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free."
    },
    video: { seconds: 39, summary: 'The ticket says solved while the customer is still stuck. Helpin keeps one history from ticket to release, with CRM built in and one price for unlimited teammates.' },
    sources: [
      {
        "label": "Zendesk pricing",
        "url": "https://www.zendesk.com/pricing/"
      },
      {
        "label": "Zendesk AI agent resolutions",
        "url": "https://support.zendesk.com/hc/en-us/articles/9570369117338"
      },
      {
        "label": "Zendesk AI allowance",
        "url": "https://support.zendesk.com/hc/en-us/articles/10479587390106"
      },
      {
        "label": "Zendesk Sell retirement",
        "url": "https://support.zendesk.com/hc/en-us/articles/9591462550042"
      },
      {
        "label": "Zendesk data export",
        "url": "https://support.zendesk.com/hc/en-us/articles/4408886165402"
      },
      {
        "label": "Zendesk Marketplace",
        "url": "https://www.zendesk.com/blog/zendesk-marketplace/"
      },
      {
        "label": "MCP client general availability",
        "url": "https://support.zendesk.com/hc/en-us/articles/11105393853210-Announcing-the-general-availability-of-the-MCP-client"
      },
      {
        "label": "Zendesk Sell retirement",
        "url": "https://support.zendesk.com/hc/en-us/articles/9591462550042-Announcing-the-retiring-of-Zendesk-Sell"
      }
    ],
  },
  {
    slug: 'help-scout',
    name: 'Help Scout',
    group: 'Customer support',
    category: 'Shared inbox',
    cardLine: "Keep support simple while AI agents investigate, follow up and work on the fix.",
    checked: "2026-10-03",
    seo: {
      "title": "Helpin vs Help Scout: Open-Source Alternative",
      "description": "Compare Helpin vs Help Scout: AI agents for support, code and docs, all your customer context in one fast workspace, and no per-seat or per-resolution fees."
    },
    hero: {
      "lede": "Keep the fast, simple inbox your team wants, and put AI agents to work on what follows. Helpin can investigate questions, create tasks, work on fixes and update docs, with conversations, deals and meetings in one place."
    },
    glance: [
      {
        "label": "Pricing",
        "competitor": "Per user, plus $0.75 per AI resolution",
        "helpin": "Per workspace, unlimited teammates, AI allowance"
      },
      {
        "label": "Beyond support",
        "competitor": "Projects and CRM through integrations",
        "helpin": "Projects, CRM and meetings built in"
      },
      {
        "label": "Hosting",
        "competitor": "Hosted by Help Scout",
        "helpin": "Open source: Cloud or your servers"
      }
    ],
    summary: {
      "title": "Keep support simple as the work grows.",
      "lede": "Helpin connects a fast inbox to agents, projects and sales. Check your messaging channels and history import needs when planning the move.",
      "competitor": [
        "You require WhatsApp, Instagram or SMS in the same inbox.",
        "You need a built-in importer for historical conversations or native mobile chat SDKs."
      ],
      "helpin": [
        "You want agents to check connected account data and act on customer requests.",
        "You want product work, sales follow-ups and meetings beside the inbox.",
        "You want imported help articles to stay current with AI-prepared updates.",
        "You want unlimited teammates and an AI allowance without per-resolution fees."
      ]
    },
    reasons: [
      {
        "title": "No per-seat or per-resolution fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan. There is no separate charge for each resolved conversation.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the work behind the reply.",
        "icon": "loop",
        "body": "A bug report can become a task with the customer’s conversation attached. Scribe plans, Forge writes and tests the code, and Lens reviews it. After the release is confirmed, configured automation can follow up with affected customers under your approval rules.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Build agents around the way you work.",
        "icon": "agents",
        "body": "Describe a job in plain language to build a custom agent or flow. Choose its tools, instructions, skills and approval rules. Agents can use connected services for account details, code and logs, so they can investigate before taking the next step.",
        "helpinLane": [
          "Your instructions",
          "Context and tools",
          "Action"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Pricing",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace",
            "competitor": "Per user: Standard $25, Plus $45 a month billed annually"
          },
          {
            "label": "Free plan",
            "helpin": "Self-hosted Community edition",
            "competitor": "Up to 5 users and 100 contacts a month"
          },
          {
            "label": "AI agent",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "AI Answers at $0.75 per resolution"
          },
          {
            "label": "Free trial",
            "helpin": "14 days, no card",
            "competitor": "15 days, no card"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Support",
        "rows": [
          {
            "label": "Channels",
            "helpin": "Web chat and email",
            "competitor": "Email, chat, WhatsApp, Instagram, Messenger, SMS"
          },
          {
            "label": "AI answers with human handoff",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Docs sites; 2 on Standard, 3 on Plus",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Widget SDKs",
            "helpin": "Web: JavaScript, React, Next.js, Vue",
            "competitor": "Web, iOS, Android"
          },
          {
            "label": "Support reporting",
            "helpin": "Knowledge-gap and project reports",
            "competitor": "Reports; history length depends on plan"
          },
          {
            "label": "Support response-time policies (SLAs)",
            "helpin": false,
            "competitor": "Limited on Standard and Plus",
            "competitorStatus": "partial"
          }
        ]
      },
      {
        "group": "Beyond support",
        "rows": [
          {
            "label": "Project management",
            "helpin": "Roadmaps, sprints, objectives, epics",
            "competitor": "Through the Jira integration on Plus",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Customer properties and company profiles",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Hosted, read-only",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "channels",
        "title": "Social messaging.",
        "body": "Help Scout supports messaging channels including WhatsApp and Instagram. Helpin supports chat and email."
      },
      {
        "icon": "import",
        "title": "Historical conversations.",
        "body": "Help Scout provides conversation importers. Helpin can import Help Scout Docs articles, but not past conversations yet."
      },
      {
        "icon": "mobile",
        "title": "Mobile chat SDKs.",
        "body": "Help Scout provides Beacon SDKs for iOS and Android. Helpin currently provides web SDKs."
      },
      {
        "icon": "billing",
        "title": "A small free Cloud inbox.",
        "body": "Help Scout’s free plan covers up to five users and 100 contacts a month. Helpin offers a Cloud trial and a free self-hosted edition."
      }
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
      [
        "Is Helpin a good Help Scout alternative?",
        "Yes. Helpin gives teams a fast shared inbox with AI agents that can investigate, follow up and carry requests into product work. It also includes projects, CRM, meetings and docs, with workspace pricing and an included AI allowance."
      ],
      [
        "Does Help Scout charge per contact?",
        "Help Scout moved to contact-based billing in 2024 and later returned to per-user pricing for new customers. Some existing accounts still use contact-based billing."
      ],
      [
        "Can Helpin import from Help Scout?",
        "Yes, for Help Scout Docs articles. Conversation history is not imported yet."
      ],
      [
        "Is Helpin cheaper than Help Scout?",
        "Helpin’s workspace price stays the same as you invite teammates, and AI answers draw from the included allowance. Help Scout can cost less for a small inbox, including on its free plan. Compare team size, AI volume and the extra project or CRM tools you need."
      ],
      [
        "Can we run Helpin alongside Help Scout?",
        "Yes. Add the Helpin widget to a few pages or forward one support address, and keep Help Scout for everything else while your team tries the workflow. Move the rest when you’re ready."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Keep the inbox simple. Put agents to work.",
      "description": "Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free."
    },
    video: { seconds: 39, summary: 'Keep a calm inbox and connect what comes next: projects, CRM, meetings and AI agents on one customer history, with AI usage included in one workspace price.' },
    sources: [
      {
        "label": "Help Scout pricing",
        "url": "https://www.helpscout.com/pricing/"
      },
      {
        "label": "Help Scout AI agent",
        "url": "https://www.helpscout.com/agent/"
      },
      {
        "label": "Help Scout contact-based billing",
        "url": "https://docs.helpscout.com/article/1593-contact-based-billing-and-plans-guide"
      },
      {
        "label": "Help Scout price and plans guide",
        "url": "https://docs.helpscout.com/article/596-price-and-plans-guide"
      },
      {
        "label": "Help Scout imports",
        "url": "https://docs.helpscout.com/article/1495-import-emails-or-tickets-into-help-scout"
      },
      {
        "label": "Help Scout MCP server",
        "url": "https://articles.helpscout.com/blog/introducing-help-scout-mcp/"
      },
      {
        "label": "Current MCP access",
        "url": "https://docs.helpscout.com/article/1779-connect-your-ai-agent-with-help-scout-to-search-conversations-and-pull-reports"
      }
    ],
  },
  {
    slug: 'chatwoot',
    name: 'Chatwoot',
    group: 'Customer support',
    category: 'Open-source support',
    cardLine: "Self-host support, coding, review and docs agents without an extra AI license.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Chatwoot Alternative: Helpin vs Chatwoot",
      "description": "Compare Helpin vs Chatwoot: AI agents for support, code and docs, all your customer context in one fast workspace, and no per-seat or per-resolution fees."
    },
    hero: {
      "lede": "Run your support inbox and the AI agents that act on it in one fast workspace. Helpin connects customer conversations to code, projects, docs and sales, with those capabilities included in the free self-hosted edition."
    },
    glance: [
      {
        "label": "License",
        "competitor": "MIT core, paid enterprise edition",
        "helpin": "AGPL-3.0, including AI agents"
      },
      {
        "label": "Self-hosted AI",
        "competitor": "Captain needs a paid plan",
        "helpin": "AI agents included"
      },
      {
        "label": "Beyond support",
        "competitor": "Issues through the Linear integration",
        "helpin": "Projects, CRM and meetings built in"
      }
    ],
    summary: {
      "title": "Choose what you want to run, not just where it runs.",
      "lede": "Helpin includes the agents and the wider workflow when you self-host. Compare the channels, license and deployment setup your team needs.",
      "competitor": [
        "You require WhatsApp, social messaging or native mobile support.",
        "Your deployment or licensing requirements call for Chatwoot’s packages or MIT-licensed core."
      ],
      "helpin": [
        "You want AI agents without a separate self-hosted AI license.",
        "You want planning, coding, review and docs updates connected to support.",
        "You want CRM, meetings and sales follow-ups in the same workspace.",
        "You want Cloud pricing per workspace, with unlimited teammates."
      ]
    },
    reasons: [
      {
        "title": "Invite the whole team without seat fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the work behind the reply.",
        "icon": "loop",
        "body": "A bug report can become a task with the customer’s conversation attached. Scribe plans, Forge writes and tests the code, and Lens reviews it. After the release is confirmed, configured automation can follow up with affected customers under your approval rules.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Self-host your agents without an AI license.",
        "icon": "open",
        "body": "Helpin’s free Community edition includes agents for support, planning, coding, review, docs and sales. Build custom agents and flows with your own instructions and skills. You control the infrastructure and AI provider, and pay for their usage directly.",
        "helpinLane": [
          "Open source",
          "Your AI provider",
          "Built-in and custom agents"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Open source and pricing",
        "rows": [
          {
            "label": "License",
            "helpin": "AGPL-3.0 for every product feature",
            "competitor": "MIT core, with a separate paid enterprise edition"
          },
          {
            "label": "AI agents included without a self-hosted license fee",
            "helpin": "Included; pay your AI provider for usage",
            "competitor": "Captain needs a paid plan from $19 an agent",
            "helpinStatus": "yes",
            "competitorStatus": "no"
          },
          {
            "label": "Cloud price model",
            "helpin": "Per workspace",
            "competitor": "Per agent: $19, $39 or $99 a month"
          },
          {
            "label": "Cloud AI",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "Captain credits, then $20 per 1,000"
          },
          {
            "label": "Free trial",
            "helpin": "14 days, no card",
            "competitor": "15 days"
          }
        ]
      },
      {
        "group": "Support",
        "rows": [
          {
            "label": "Channels",
            "helpin": "Web chat and email",
            "competitor": "Chat, email, WhatsApp, social, Telegram, LINE, SMS, voice"
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Built in, with search and locales",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Widget SDKs",
            "helpin": "Web: JavaScript, React, Next.js, Vue",
            "competitor": "Web, React Native, Flutter"
          },
          {
            "label": "Support reporting",
            "helpin": "Knowledge-gap and project reports",
            "competitor": "Agent, team, inbox and CSAT reports"
          },
          {
            "label": "Company single sign-on (SAML)",
            "helpin": false,
            "competitor": "Enterprise plan",
            "competitorStatus": "yes"
          },
          {
            "label": "Support response-time policies (SLAs)",
            "helpin": false,
            "competitor": "Enterprise plan",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Beyond support",
        "rows": [
          {
            "label": "Project management",
            "helpin": "Roadmaps, sprints, objectives, epics",
            "competitor": "Through the Linear integration",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Contacts, companies and segments",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "External agents can use the CLI; project workflow is separate",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Community-built only",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "channels",
        "title": "Additional inbox channels.",
        "body": "Chatwoot supports WhatsApp, social messaging, Telegram, LINE, SMS and voice. Helpin supports chat and email."
      },
      {
        "icon": "deploy",
        "title": "Deployment packages.",
        "body": "Chatwoot provides Docker, Kubernetes Helm charts and other deployment packages. Helpin documents self-hosting with Docker Compose."
      },
      {
        "icon": "mobile",
        "title": "Native mobile clients.",
        "body": "Chatwoot offers iOS and Android apps and mobile widget SDKs. Helpin currently provides web access and web SDKs."
      },
      {
        "icon": "open",
        "title": "License requirements.",
        "body": "Chatwoot’s core uses MIT with separate paid enterprise features. Helpin uses AGPL-3.0 for its product features, including agents."
      }
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
      [
        "Are Helpin and Chatwoot both open source?",
        "Yes. Chatwoot’s core is MIT-licensed with a separate paid enterprise edition. Helpin’s product features are open source under AGPL-3.0."
      ],
      [
        "Which is better for self-hosting?",
        "Choose Helpin if you want support, projects, CRM, docs and AI agents included without a license fee. You run it with Docker Compose and your own AI provider. Chatwoot offers additional deployment packages, but Captain requires a paid self-hosted plan."
      ],
      [
        "Can I use my own AI provider?",
        "Yes, with both. Self-hosted Helpin connects to OpenAI, Anthropic, OpenRouter or an OpenAI-compatible endpoint. Chatwoot’s Captain supports your own OpenAI-compatible key on a paid plan."
      ],
      [
        "Does Helpin support WhatsApp?",
        "Not today. Helpin supports web chat and email. If WhatsApp or social channels are essential, Chatwoot covers them."
      ],
      [
        "Can we run Helpin alongside Chatwoot?",
        "Yes. Add the Helpin widget to a few pages or forward one support address, and keep Chatwoot for everything else while your team tries the workflow. Move the rest when you’re ready."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Self-host the agents that move work forward.",
      "description": "Self-host the Community edition for free, or start a 14-day trial of Helpin Cloud with no card."
    },
    video: { seconds: 37, summary: 'Both are open source. Chatwoot’s AI agent needs a paid plan even when self-hosted; Helpin’s free Community edition includes AI agents, projects and CRM.' },
    sources: [
      {
        "label": "Chatwoot pricing",
        "url": "https://www.chatwoot.com/pricing"
      },
      {
        "label": "Chatwoot self-hosted plans",
        "url": "https://www.chatwoot.com/pricing/self-hosted-plans"
      },
      {
        "label": "Chatwoot license",
        "url": "https://github.com/chatwoot/chatwoot/blob/develop/LICENSE"
      },
      {
        "label": "Chatwoot enterprise edition",
        "url": "https://developers.chatwoot.com/self-hosted/enterprise-edition"
      },
      {
        "label": "Captain on self-hosted installs",
        "url": "https://www.chatwoot.com/hc/user-guide/articles/1755284287-how-to-enable-captain-on-self_hosted-installations"
      },
      {
        "label": "Chatwoot features",
        "url": "https://www.chatwoot.com/features"
      },
      {
        "label": "Chatwoot on GitHub",
        "url": "https://github.com/chatwoot/chatwoot"
      },
      {
        "label": "CLI for external AI agents",
        "url": "https://www.chatwoot.com/agents"
      }
    ],
  },
  {
    "slug": "chatbase",
    "name": "Chatbase",
    "group": "Customer support",
    "category": "AI agents and helpdesk",
    "cardLine": "Take customer requests through code, docs and sales with agents in one fast workspace.",
    "checked": "2026-10-03",
    "seo": {
      "title": "Open-Source Chatbase Alternative: Helpin vs Chatbase",
      "description": "Compare Helpin vs Chatbase: AI support, coding agents, current docs and CRM in one fast workspace. Unlimited teammates, shared context and self-hosting."
    },
    "hero": {
      "lede": "Give your AI agents the customer history and the tools to finish the work. Helpin connects support to planning, code review, docs and sales in one fast workspace, with unlimited teammates and the option to self-host."
    },
    "glance": [
      {
        "label": "Team access",
        "competitor": "Members included up to each plan’s limit",
        "helpin": "Unlimited teammates on every plan"
      },
      {
        "label": "Agent work",
        "competitor": "Customer-facing agents, procedures and helpdesk",
        "helpin": "Support, planning, coding, review, docs and sales"
      },
      {
        "label": "Hosting",
        "competitor": "Chatbase Cloud",
        "helpin": "Cloud or open-source self-hosting"
      }
    ],
    "summary": {
      "title": "Give agents the customer request and the work behind it.",
      "lede": "Helpin connects the support conversation to the people and agents who can act on it. Check the channels and integrations you need before moving.",
      "helpin": [
        "You want AI agents to investigate a report, write code and prepare it for review.",
        "You want guides and internal docs to keep up with releases and unanswered questions.",
        "You want customer conversations, deals and meetings in the same fast workspace.",
        "You want unlimited teammates and an open-source option you can run yourself."
      ],
      "competitor": [
        "Your workflow depends on Chatbase’s voice, telephony or social channels.",
        "You want to keep a specific helpdesk integration and use Chatbase’s procedures on top of it."
      ]
    },
    "reasons": [
      {
        "title": "Give agents the work after the answer.",
        "icon": "loop",
        "body": "A customer report can become a task with the conversation attached. Scribe plans, Forge writes and tests the code, and Lens reviews it. Your team can follow the investigation and approve the change without rebuilding the customer’s story in another tool.",
        "helpinLane": [
          "Conversation",
          "Task",
          "Agent work",
          "Review"
        ]
      },
      {
        "title": "Keep the source of the answer current.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture screenshots and browser recordings of the current product too. Your team reviews and publishes the updates, so customers and agents have instructions they can use.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Docs update"
        ]
      },
      {
        "title": "Bring the whole team into the workspace.",
        "icon": "team",
        "body": "Chatbase’s published plans include a set number of members. Helpin includes unlimited teammates, so support, developers and sales can share the context and review agent work. Every Cloud plan includes an AI allowance, with extra usage metered only when you enable it.",
        "helpinLane": [
          "Support",
          "Development",
          "Sales",
          "Shared context"
        ]
      },
      {
        "title": "Build agents for more of your business.",
        "icon": "agents",
        "body": "Describe the job to build a custom agent or flow. Choose its tools, skills and approval rules, and connect external AI tools through MCP. Helpin’s free self-hosted edition includes these agent capabilities, with your choice of infrastructure and AI provider.",
        "helpinLane": [
          "Your instructions",
          "Tools and skills",
          "Agent work"
        ]
      }
    ],
    "tableLede": "Compare the work agents can carry out, the customer context they can use and the tools your team gets in the same product.",
    "table": [
      {
        "group": "Pricing and access",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace, unlimited teammates",
            "competitor": "Workspace plans with member and AI-credit limits"
          },
          {
            "label": "Published monthly plans",
            "helpin": "Starter $99; Growth $299",
            "competitor": "Hobby $40; Standard $150; Pro $500"
          },
          {
            "label": "Included teammates",
            "helpin": "Unlimited",
            "competitor": "Hobby 2; Standard 3; Pro 5"
          },
          {
            "label": "AI billing",
            "helpin": "Included usage allowance; optional metered extra usage",
            "competitor": "Message credits; more credits available to buy"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Support and knowledge",
        "rows": [
          {
            "label": "Channels",
            "helpin": "Web chat and email",
            "competitor": "Website, email, social channels and voice"
          },
          {
            "label": "AI answers and human handoff",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "Connected data and actions",
            "helpin": "Selected tools for account data, code and logs",
            "competitor": "Procedures, integrations and custom actions",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Help center",
            "helpin": "Articles, search, AI answers and API reference",
            "competitor": "Agent Page answers from connected documentation",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Docs upkeep",
            "helpin": "Agent-prepared updates with screenshots and browser recordings",
            "competitor": "Source resync and suggestions"
          },
          {
            "label": "SDKs and APIs",
            "helpin": "JavaScript, React, Next.js, Vue and APIs",
            "competitor": "Web, voice, Android and iOS SDKs; APIs"
          }
        ]
      },
      {
        "group": "The work beyond a reply",
        "rows": [
          {
            "label": "Project planning",
            "helpin": "Objectives, roadmaps, epics, sprints and tasks",
            "competitor": "Separate project tools",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "Separate coding and project workflow",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Lead capture and CRM integrations",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes and next steps",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": "Separate meeting tools",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP access for external agents",
            "helpin": "Read context, update tasks/docs and start agent runs",
            "competitor": "Manage agents, sources, conversations and tickets",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    "strengths": [
      {
        "icon": "channels",
        "title": "Voice and social channels.",
        "body": "Chatbase supports voice, telephony and messaging channels. Helpin’s shared inbox currently supports web chat and email."
      },
      {
        "icon": "mobile",
        "title": "Native mobile SDKs.",
        "body": "Chatbase publishes Android and iOS SDKs. Helpin currently provides web SDKs; check the mobile experience your product needs."
      },
      {
        "icon": "ecosystem",
        "title": "An existing helpdesk setup.",
        "body": "Chatbase connects to helpdesks such as Zendesk and Salesforce. Review any specific integration your team relies on when planning a move."
      },
      {
        "icon": "enterprise",
        "title": "Company access requirements.",
        "body": "Chatbase lists SSO and audit logs on Enterprise. Check the exact identity setup you need; Helpin does not currently provide SAML sign-in."
      }
    ],
    "switching": {
      "title": "Move one real workflow into Helpin.",
      "lede": "Start with a support address or a few product pages, then compare the whole path from question to completed work.",
      "take": [
        "Your source articles, files and customer FAQs",
        "Agent instructions and the actions you want to keep",
        "A record of conversations you need to retain"
      ],
      "setUp": [
        "Helpin chat widget and support email",
        "Help center and internal docs spaces",
        "Echo’s allowed tools and handoff rules",
        "Repository access for coding and review agents"
      ],
      "steps": [
        {
          "title": "Bring over your source content",
          "body": "Use the articles, files and instructions behind your Chatbase agent to set up Helpin’s knowledge. There is no direct Chatbase importer today.",
          "status": "Available now"
        },
        {
          "title": "Try a customer issue from start to finish",
          "body": "Ask Echo to investigate, link the report to a task and have coding agents prepare a change for review. Configure the tools and approvals first.",
          "status": "Available now"
        },
        {
          "title": "Move conversations at your pace",
          "body": "Run both products while you move selected pages or support addresses. Keep historical conversations available in Chatbase until your retention needs are covered.",
          "status": "Available now"
        }
      ]
    },
    "faqs": [
      [
        "Is Helpin a Chatbase alternative?",
        "Yes. Helpin combines AI support with projects, coding and review agents, docs, CRM and meetings. It is a strong fit when customer questions become work across the business, and you want your team and agents to use the same context."
      ],
      [
        "How are Helpin’s agents different from Chatbase’s agents?",
        "Chatbase supports customer-facing agents with procedures, connected actions and human handoff. Helpin also includes agents that plan tasks, write and review code, maintain guides and follow up on sales. You can build custom agents and flows with selected tools and skills."
      ],
      [
        "Does Chatbase charge per resolution?",
        "Its published plans use message credits, rather than a fee per resolved conversation. Helpin includes an AI usage allowance too. Compare how much work the allowance covers, the included members and any extra tools you need."
      ],
      [
        "Can I keep using external AI agents?",
        "Yes. Both products offer MCP access. In Helpin, compatible tools such as Claude Code, Codex, Cursor and Hermes can find context, update work and start agent runs with the permissions you allow."
      ],
      [
        "Can Helpin import my Chatbase workspace?",
        "There is no direct Chatbase importer today. You can bring over source content and instructions, configure the actions you want to keep and run both products during the move."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    "closing": {
      "title": "Turn the customer’s question into completed work.",
      "description": "Try Helpin Cloud for 14 days with no card, or self-host the open-source edition with your own AI provider."
    },
    "video": {
      "seconds": 62.4,
      "summary": "See Helpin’s connected workspace for customer support, projects, CRM, docs and AI agents.",
      "media": {
        "title": "See Helpin in action",
        "src": "/new/home/helpin-launch-1080p-v1.mp4",
        "poster": "/new/home/helpin-launch-poster-1600-v3.webp",
        "published": "2026-10-02T16:50:29Z"
      }
    },
    "sources": [
      {
        "label": "Chatbase pricing and included members",
        "url": "https://www.chatbase.co/pricing"
      },
      {
        "label": "Chatbase product overview",
        "url": "https://www.chatbase.co/features/product-overview"
      },
      {
        "label": "Chatbase helpdesk and channels",
        "url": "https://www.chatbase.co/features/helpdesk"
      },
      {
        "label": "Chatbase procedures and actions",
        "url": "https://www.chatbase.co/features/procedures"
      },
      {
        "label": "Chatbase Agent Page and source docs",
        "url": "https://www.chatbase.co/blog/introducing-agent-page"
      },
      {
        "label": "Chatbase MCP access",
        "url": "https://www.chatbase.co/docs/developer-guides/mcp"
      },
      {
        "label": "Chatbase Android SDK",
        "url": "https://github.com/Chatbase-co/chatbase-android"
      }
    ]
  },
  {
    "slug": "crisp",
    "name": "Crisp",
    "group": "Customer support",
    "category": "Shared inbox and Hugo AI",
    "cardLine": "Connect AI support to product work, docs and sales with unlimited teammates.",
    "checked": "2026-10-03",
    "seo": {
      "title": "Open-Source Crisp Alternative: Helpin vs Crisp",
      "description": "Compare Helpin vs Crisp: fast AI support, unlimited teammates and agents for code, docs and sales. See pricing, customer context and self-hosting options."
    },
    "hero": {
      "lede": "Bring the conversation and the work it creates into one fast workspace. Helpin’s AI agents can answer customers, work on fixes, update guides and follow up, with projects, CRM and meetings built in for your whole team."
    },
    "glance": [
      {
        "label": "Pricing",
        "competitor": "Per workspace, with included seats",
        "helpin": "Per workspace, unlimited teammates"
      },
      {
        "label": "AI work",
        "competitor": "Hugo support agent and task automations",
        "helpin": "Support, planning, coding, review, docs and sales"
      },
      {
        "label": "Hosting",
        "competitor": "Crisp Cloud",
        "helpin": "Cloud or open-source self-hosting"
      }
    ],
    "summary": {
      "title": "Keep the inbox connected to the work that follows.",
      "lede": "Helpin brings product delivery, sales and current docs into the same workspace as support. Check the channels and customer-facing tools you depend on.",
      "helpin": [
        "You want agents to investigate customer reports and prepare fixes for review.",
        "You want support, projects, deals and meetings in one fast workspace.",
        "You want agents to keep help articles and internal docs current as you ship.",
        "You want unlimited teammates and the option to self-host your agents and data."
      ],
      "competitor": [
        "You require Crisp’s social channels, native mobile apps or mobile chat SDKs.",
        "Your team depends on Crisp’s customer portal or a specific existing integration."
      ]
    },
    "reasons": [
      {
        "title": "Carry the conversation through to the fix.",
        "icon": "loop",
        "body": "A bug report should reach the agents and people who can solve it. In Helpin, the conversation can become a task with the relevant context attached. Planning, coding and review agents can take on the work while your team follows the progress and approves the change.",
        "helpinLane": [
          "Report",
          "Investigation",
          "Code",
          "Review"
        ]
      },
      {
        "title": "Bring everyone in without counting seats.",
        "icon": "team",
        "body": "Crisp already charges per workspace, with seats included by plan. Helpin includes unlimited teammates on every plan. Support, developers and sales can all review the same history, and AI agents can act across the connected work.",
        "helpinLane": [
          "Support",
          "Development",
          "Sales",
          "One workspace"
        ]
      },
      {
        "title": "Keep customers and agents on current instructions.",
        "icon": "docs",
        "body": "Quill can prepare updates for your help center and internal docs when the product changes or support reveals a missing answer. Fresh screenshots and browser recordings show the current steps. Your team reviews and publishes the changes.",
        "helpinLane": [
          "Knowledge gap",
          "UI capture",
          "Reviewed update"
        ]
      },
      {
        "title": "Give agents the whole account context.",
        "icon": "crm",
        "body": "Deals, conversations and recorded meetings sit beside the work customers are waiting for. Beacon can identify buying intent and carry out approved follow-ups. Custom agents and flows can use the same context, with the tools, skills and permissions you choose.",
        "helpinLane": [
          "Conversation",
          "Deal and meeting",
          "Next action"
        ]
      }
    ],
    "tableLede": "Both products combine a shared inbox with AI. Compare what else the agents and your team can do in the same workspace.",
    "table": [
      {
        "group": "Pricing and access",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace, unlimited teammates",
            "competitor": "Per workspace, seats included by plan"
          },
          {
            "label": "Published monthly plans",
            "helpin": "Starter $99; Growth $299",
            "competitor": "Mini $45; Essentials $95; Plus $295"
          },
          {
            "label": "Included teammates",
            "helpin": "Unlimited",
            "competitor": "Mini 4; Essentials 10; Plus 20, then extra seats"
          },
          {
            "label": "AI billing",
            "helpin": "Included usage allowance; optional metered extra usage",
            "competitor": "Included Hugo credits; optional pay-as-you-go"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Support and knowledge",
        "rows": [
          {
            "label": "Shared chat and email inbox",
            "helpin": true,
            "competitor": "Chat; shared email on paid plans",
            "competitorStatus": "yes"
          },
          {
            "label": "Additional channels",
            "helpin": "Web chat and email",
            "competitor": "WhatsApp, social messaging, SMS and phone connections"
          },
          {
            "label": "AI answers and human handoff",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "Connected data and actions",
            "helpin": "Selected tools for account details, code and logs",
            "competitor": "Hugo tools, workflows and external MCP services",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Public help center",
            "helpin": "Articles, AI answers, own domain and API reference",
            "competitor": "Knowledge base on Essentials and Plus",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Native mobile apps and SDKs",
            "helpin": false,
            "competitor": "Mobile apps and iOS/Android chat SDKs",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "The work beyond a reply",
        "rows": [
          {
            "label": "Project planning",
            "helpin": "Objectives, roadmaps, epics, sprints and tasks",
            "competitor": "Separate project tools",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "Separate coding and project workflow",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": "Customer profiles and connected sales CRMs",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Meeting notes and next steps",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": "Separate meeting tools",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP connections",
            "helpin": "External tools; agents can read context and update work",
            "competitor": "External tools for Hugo; server for knowledge search",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    "strengths": [
      {
        "icon": "channels",
        "title": "Messaging and phone channels.",
        "body": "Crisp connects WhatsApp, social messaging, SMS and phone tools. Helpin’s shared inbox currently supports web chat and email."
      },
      {
        "icon": "mobile",
        "title": "Native mobile support.",
        "body": "Crisp provides mobile apps for teammates and native chat SDKs. Helpin currently provides a web workspace and web SDKs."
      },
      {
        "icon": "requests",
        "title": "A customer ticket portal.",
        "body": "Crisp Plus includes a portal where customers can open and follow tickets. Check whether this is part of the support experience you need."
      },
      {
        "icon": "ecosystem",
        "title": "Existing integrations.",
        "body": "Review any Crisp connection your team depends on, such as an ecommerce platform or a phone provider, before moving that workflow."
      }
    ],
    "switching": {
      "title": "Start with one inbox and a real customer issue.",
      "lede": "Move a small part of support first, then try the connected agent workflow with your team.",
      "take": [
        "Help articles, saved replies and workflow instructions",
        "Customer fields and records you need to keep",
        "A record of channels and integrations your team uses"
      ],
      "setUp": [
        "Helpin chat widget and support address",
        "Team inboxes and agent handoff rules",
        "Help center and internal docs spaces",
        "Projects, repository access and customer records"
      ],
      "steps": [
        {
          "title": "Set up the knowledge and inbox",
          "body": "Bring over the guides and instructions your team uses to answer customers. Add a support address and configure Echo’s tools and permissions.",
          "status": "Available now"
        },
        {
          "title": "Try the work behind the conversation",
          "body": "Take a real report into a task, ask agents to investigate and prepare a fix, then review the result. Set up follow-ups for the conversations you want agents to handle.",
          "status": "Available now"
        },
        {
          "title": "Move the remaining workflows gradually",
          "body": "There is no direct Crisp importer today. Keep older conversations and any required channels in Crisp while you move the work Helpin will handle.",
          "status": "Available now"
        }
      ]
    },
    "faqs": [
      [
        "Is Helpin a Crisp alternative?",
        "Yes. Helpin combines a fast chat and email inbox with agents for support, planning, coding, review, docs and sales. It also includes projects, CRM and meeting notes, so customer conversations stay connected to the work they create."
      ],
      [
        "Doesn’t Crisp already charge per workspace?",
        "Yes. Crisp uses workspace pricing and bundles seats by plan. Helpin’s difference is unlimited teammates on every plan, plus built-in projects, coding and review agents, CRM and meetings. Compare the complete workflow your team needs, as well as the inbox price."
      ],
      [
        "How does Helpin compare with Hugo?",
        "Hugo can answer customers and take actions through connected tools. Helpin’s Echo agent handles support too, alongside agents that plan and code fixes, review changes, maintain docs and act on sales follow-ups. The work shares customer context within Helpin."
      ],
      [
        "Do both include AI usage?",
        "Yes. Crisp’s paid plans include Hugo credits, with optional pay-as-you-go. Helpin’s Cloud plans include an AI allowance, with optional metered extra usage. Neither offer should be treated as unlimited AI."
      ],
      [
        "Can Helpin replace every Crisp channel?",
        "Helpin currently handles web chat and email. If you use WhatsApp, social messaging or phone integrations in Crisp, plan how those channels will be handled before moving."
      ],
      [
        "Can Helpin import my Crisp conversations?",
        "There is no direct Crisp conversation importer today. You can bring over source content and instructions and start Helpin alongside Crisp while you plan the rest of the move."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    "closing": {
      "title": "Put agents to work across the whole customer story.",
      "description": "Try Helpin Cloud for 14 days with no card, or self-host the open-source edition with your own AI provider."
    },
    "video": {
      "seconds": 62.4,
      "summary": "See Helpin’s connected workspace for customer support, projects, CRM, docs and AI agents.",
      "media": {
        "title": "See Helpin in action",
        "src": "/new/home/helpin-launch-1080p-v1.mp4",
        "poster": "/new/home/helpin-launch-poster-1600-v3.webp",
        "published": "2026-10-02T16:50:29Z"
      }
    },
    "sources": [
      {
        "label": "Crisp pricing, included seats and channels",
        "url": "https://crisp.chat/en/pricing/"
      },
      {
        "label": "Hugo usage and pay-as-you-go",
        "url": "https://help.crisp.chat/en/article/how-does-hugo-ai-pricing-and-billing-works-h78hkv/"
      },
      {
        "label": "Hugo tools and MCP connections",
        "url": "https://docs.crisp.chat/guides/hugo/"
      },
      {
        "label": "Hugo MCP server capabilities",
        "url": "https://docs.crisp.chat/guides/hugo/mcp-server/"
      },
      {
        "label": "Crisp chatbox SDKs",
        "url": "https://docs.crisp.chat/guides/chatbox-sdks/"
      },
      {
        "label": "Crisp customer records and CRM integrations",
        "url": "https://crisp.chat/en/crm/"
      },
      {
        "label": "Crisp inbox workflows and support tools",
        "url": "https://help.crisp.chat/en/article/how-to-include-crisp-as-part-of-a-website-contract-13m869q/"
      }
    ]
  },
  {
    slug: 'linear',
    name: 'Linear',
    group: 'Project management',
    category: 'Project management',
    cardLine: "Fast project work with agents, customer conversations and account context built in.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Linear Alternative: Helpin vs Linear",
      "description": "Compare Helpin vs Linear: fast work, AI agents, shared customer context and no per-seat fees. Explore support, projects, docs, CRM and self-hosting."
    },
    hero: {
      "lede": "Move quickly from a customer request to a reviewed change. Helpin combines fast project planning with AI agents, a shared inbox, CRM, meetings and docs, so the team and its agents can work from the whole customer context."
    },
    glance: [
      {
        "label": "Customer requests",
        "competitor": "Linked from Intercom or Zendesk",
        "helpin": "Built-in support conversations and linked tasks"
      },
      {
        "label": "Pricing",
        "competitor": "Per user",
        "helpin": "One workspace price, unlimited teammates"
      },
      {
        "label": "Hosting",
        "competitor": "Hosted by Linear",
        "helpin": "Open source: Cloud or your servers"
      }
    ],
    summary: {
      "title": "Keep the speed. Bring the customer into the workflow.",
      "lede": "Helpin combines structured planning and agent delivery with support and sales. Check any specific Linear integration your team depends on.",
      "competitor": [
        "Your team needs Linear’s native mobile apps or a specific existing integration.",
        "You require its SAML sign-in or a built-in importer for your current issue tracker."
      ],
      "helpin": [
        "You want a fast workspace for roadmaps, objectives, epics, sprints and tasks.",
        "You want agents to plan, code and review with the original customer conversation attached.",
        "You want built-in support, CRM, meetings and AI-maintained docs.",
        "You want built-in, custom or external agents, without per-user pricing."
      ]
    },
    reasons: [
      {
        "title": "Invite the whole team without seat fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the context to build the right thing.",
        "icon": "loop",
        "body": "A task can carry the original conversation, account history and linked docs. Planning and coding agents can use that context, plus your repository and connected tools, to investigate and prepare a fix. Your team sees the work and reviews the change.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Build agents around the way you work.",
        "icon": "agents",
        "body": "Use built-in planning, coding and review agents, or build a custom agent in plain language. Connect Claude Code, Codex, Cursor, Hermes and other compatible tools through MCP. Helpin’s agents can also use external systems with the permissions you allow.",
        "helpinLane": [
          "Your instructions",
          "Context and tools",
          "Action"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Pricing",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace",
            "competitor": "Per user: Basic $10, Business $16 a month billed annually"
          },
          {
            "label": "Free plan",
            "helpin": "Self-hosted Community edition",
            "competitor": "Unlimited members, 2 teams, 250 issues"
          },
          {
            "label": "AI coding",
            "helpin": "From the AI allowance included in every Cloud plan",
            "competitor": "Prepaid AI credits for coding sessions"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": false,
            "helpinStatus": "yes"
          }
        ]
      },
      {
        "group": "Planning",
        "rows": [
          {
            "label": "Roadmaps and sprints",
            "helpin": true,
            "competitor": "Initiatives, projects and cycles",
            "competitorStatus": "yes"
          },
          {
            "label": "Triage",
            "helpin": true,
            "competitor": true
          },
          {
            "label": "GitHub and GitLab",
            "helpin": "PR and MR linking",
            "competitor": "PR and MR linking",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "Linear Agent plus Cursor, Codex, Copilot and others",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Delivery reporting",
            "helpin": "Velocity and sprint reports",
            "competitor": "Insights and dashboards on Business",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Native mobile apps",
            "helpin": false,
            "competitor": "iOS and Android",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Customers",
        "rows": [
          {
            "label": "Support inbox and live chat",
            "helpin": "Web chat and email",
            "competitor": "Integrations with Intercom and Zendesk on Business",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": "Gong transcripts on Enterprise",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Hosted",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "mobile",
        "title": "Native mobile apps.",
        "body": "Linear offers native iOS and Android apps. Helpin currently provides a web workspace."
      },
      {
        "icon": "enterprise",
        "title": "Company single sign-on.",
        "body": "Linear offers SAML on Enterprise. Helpin does not currently provide SAML sign-in."
      },
      {
        "icon": "import",
        "title": "Historical issue imports.",
        "body": "Linear provides importers for several trackers. Helpin imports Shortcut projects today; a Linear importer is not available yet."
      },
      {
        "icon": "ecosystem",
        "title": "Existing team integrations.",
        "body": "Check the exact Linear apps your team uses. Helpin connects to GitHub, GitLab and external tools through supported integrations and MCP."
      }
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
      lede: 'Start with one project or support inbox, then move more work when your team is ready.',
      take: ['Issues, as CSV or through the API', 'Projects and cycles you want to plan in Helpin', 'Specs and docs'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Linear connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Linear, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Linear’s tools through MCP, so engineering can stay in Linear while support runs in Helpin.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Linear issues', status: 'Not yet', body: 'There is no Linear importer yet. Linear exports issues to CSV and through its API. Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      [
        "Is Helpin a Linear alternative?",
        "Yes. Helpin includes fast project planning with objectives, roadmaps, epics, sprints, dependencies and reports. Agents can plan, code and review while support conversations, deals, docs and meeting context stay in the same product."
      ],
      [
        "Can I use Helpin and Linear together?",
        "Yes. Helpin’s agents can use Linear’s tools through MCP, so support can run in Helpin while engineering stays in Linear."
      ],
      [
        "Does Linear have customer support features?",
        "Linear has no customer-facing inbox, live chat or help center. It links customer requests from support tools such as Intercom and Zendesk."
      ],
      [
        "Do both have coding agents?",
        "Yes. Linear supports its own coding sessions and connected agents. Helpin has built-in planning, coding and review agents, plus custom agents and MCP access for compatible external tools. Helpin also connects that work to support, docs and sales follow-ups."
      ],
      [
        "Can Helpin import from Linear?",
        "Not yet. Linear exports issues to CSV and through its API, and Helpin imports projects from Shortcut today."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Give your agents the customer context and the work.",
      "description": "Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free."
    },
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
    cardLine: "Open-source planning with agents, customer support and sales in the same workspace.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Plane Alternative: Helpin vs Plane",
      "description": "Compare Helpin vs Plane: fast work, AI agents, shared customer context and no per-seat fees. Explore support, projects, docs, CRM and self-hosting."
    },
    hero: {
      "lede": "Plan the work, give it to AI agents and review the result in one fast workspace. Helpin connects roadmaps and sprints to customer conversations, code, docs and deals, with AI agents included when you self-host."
    },
    glance: [
      {
        "label": "Customer support",
        "competitor": "Help desk coming soon; Intake forms on Business",
        "helpin": "Chat, email, help center and AI agents built in"
      },
      {
        "label": "Open-source edition",
        "competitor": "Free-plan features; AI needs the paid edition",
        "helpin": "Every product feature, AI agents included"
      },
      {
        "label": "Pricing",
        "competitor": "Per seat, with AI credits per seat",
        "helpin": "One workspace price, AI usage included"
      }
    ],
    summary: {
      "title": "Bring planning and customer work together.",
      "lede": "Helpin combines project structure with agents that work across the business. Check your deployment, identity and migration requirements separately.",
      "competitor": [
        "You require a packaged Kubernetes or air-gapped Enterprise deployment.",
        "You need SAML sign-in, native mobile apps or one of Plane’s existing importers."
      ],
      "helpin": [
        "You want objectives, roadmaps, epics and sprints connected to a live support inbox.",
        "You want agents for planning, coding, review, docs and sales in the free self-hosted edition.",
        "You want deals and meeting decisions available beside the work.",
        "You want a fast workspace with unlimited teammates on Cloud."
      ]
    },
    reasons: [
      {
        "title": "Invite the whole team without seat fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the context to build the right thing.",
        "icon": "loop",
        "body": "A task can carry the original conversation, account history and linked docs. Planning and coding agents can use that context, plus your repository and connected tools, to investigate and prepare a fix. Your team sees the work and reviews the change.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep the guides current as you ship.",
        "icon": "docs",
        "body": "Quill can update your help center and internal docs from released changes and unanswered questions. It can capture fresh screenshots and browser recordings too. Your team reviews and publishes the changes, so customers and agents have current instructions.",
        "helpinLane": [
          "Release",
          "UI capture",
          "Article update"
        ]
      },
      {
        "title": "Self-host your agents without an AI license.",
        "icon": "open",
        "body": "Helpin’s free Community edition includes agents for support, planning, coding, review, docs and sales. Build custom agents and flows with your own instructions and skills. You control the infrastructure and AI provider, and pay for their usage directly.",
        "helpinLane": [
          "Open source",
          "Your AI provider",
          "Built-in and custom agents"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Open source and pricing",
        "rows": [
          {
            "label": "License",
            "helpin": "AGPL-3.0 for every product feature",
            "competitor": "AGPL-3.0 Community Edition; paid features in a closed-source Commercial Edition"
          },
          {
            "label": "AI agents included without a self-hosted license fee",
            "helpin": "Included; pay your AI provider for usage",
            "competitor": "Commercial Edition, with your own AI provider",
            "helpinStatus": "yes",
            "competitorStatus": "no"
          },
          {
            "label": "Cloud price model",
            "helpin": "Per workspace",
            "competitor": "Per seat: Pro $6, Business $13 a month billed annually"
          },
          {
            "label": "Free plan",
            "helpin": "Self-hosted Community edition",
            "competitor": "Up to 12 seats on Cloud, and the self-hosted Community Edition"
          },
          {
            "label": "Cloud AI",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "Personal allowances and a shared agent pool on paid plans"
          },
          {
            "label": "Free trial",
            "helpin": "14 days, no card",
            "competitor": "14 days of Business"
          }
        ]
      },
      {
        "group": "Planning",
        "rows": [
          {
            "label": "Roadmaps and sprints",
            "helpin": true,
            "competitor": "Cycles, modules, initiatives and milestones",
            "competitorStatus": "yes"
          },
          {
            "label": "Triage",
            "helpin": true,
            "competitor": "Intake; forms and email on Business",
            "competitorStatus": "yes"
          },
          {
            "label": "GitHub and GitLab",
            "helpin": "PR and MR linking",
            "competitor": "On Pro and above",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "Assign work items to Cursor on Pro and above",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Native mobile apps",
            "helpin": false,
            "competitor": "iOS and Android",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Customers",
        "rows": [
          {
            "label": "Support inbox and live chat",
            "helpin": "Web chat and email",
            "competitor": "Help desk coming soon",
            "helpinStatus": "yes",
            "competitorStatus": "no"
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Published pages on Pro",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Customer profiles",
            "helpin": "Contacts and companies with every conversation and task",
            "competitor": "Customers on Business",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": "Search Granola notes through Plane AI",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Hosted for Plane Cloud, or run locally",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "deploy",
        "title": "Deployment requirements.",
        "body": "Plane provides Kubernetes and an air-gapped Enterprise edition. Helpin’s documented setup uses Docker Compose."
      },
      {
        "icon": "enterprise",
        "title": "Identity requirements.",
        "body": "Plane offers SAML and other company sign-in options on paid plans. Helpin does not currently provide SAML."
      },
      {
        "icon": "mobile",
        "title": "Native apps.",
        "body": "Plane provides mobile and desktop apps. Helpin currently provides a web workspace."
      },
      {
        "icon": "import",
        "title": "Historical work imports.",
        "body": "Plane imports from several project tools. Helpin imports Shortcut projects today, but does not have a Plane importer yet."
      }
    ],
    calculator: {
      "seatsLabel": "People on the team",
      "seatsUnit": [
        "person",
        "people"
      ],
      "seats": 20,
      "plans": [
        {
          "name": "Pro",
          "annual": 6,
          "monthly": 8
        },
        {
          "name": "Business",
          "annual": 13,
          "monthly": 15
        }
      ],
      "plan": 1,
      "helpinPlan": "growth",
      "notes": [
        "Plane Business includes Customers and intake forms. Paid plans include personal AI allowances and a separate shared budget for agents; no additional AI usage is priced here.",
        "For small teams on Pro, Plane costs less than Helpin, and its free plan covers up to 12 seats."
      ]
    },
    switching: {
      title: 'Moving from Plane.',
      lede: 'Start with one project or support inbox, then move more work when your team is ready.',
      take: ['Work items, as CSV, Excel or JSON', 'Pages you want to keep as docs', 'Cycles and modules you want to plan in Helpin'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Plane connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Plane, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Plane Cloud’s tools through its hosted MCP server, so engineering can stay in Plane while support runs in Helpin.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Plane work items', status: 'Not yet', body: 'There is no Plane importer yet. Plane exports work items as CSV, Excel or JSON, and Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      [
        "Is Helpin a Plane alternative?",
        "Yes. Helpin combines fast project planning with AI agents, support, CRM, docs and meeting notes. Its free self-hosted edition includes agents for planning, coding, review and customer work; Cloud pricing is per workspace."
      ],
      [
        "Are Helpin and Plane both open source?",
        "Yes. Plane’s Community Edition is AGPL-3.0 and matches its Free plan; its paid features run in a closed-source Commercial Edition. Every Helpin product feature is open source under AGPL-3.0."
      ],
      [
        "Which is better for self-hosting?",
        "Choose Helpin if you want support, projects, CRM, docs and AI agents included without a license fee. You run it with Docker Compose and your own AI provider. Plane offers Kubernetes and an air-gapped Enterprise edition; AI runs in its commercial edition."
      ],
      [
        "Does Plane have a help desk?",
        "Plane lists its Desk help desk as coming soon. Today it collects requests through Intake, with public forms and email on Business, and links them to customer profiles with Customers on Business."
      ],
      [
        "Can Helpin import from Plane?",
        "Not yet. Plane exports work items as CSV, Excel or JSON, and Helpin imports projects from Shortcut today."
      ],
      [
        "Can we run Helpin alongside Plane?",
        "Yes. Add the Helpin widget to a few pages or forward one support address, and keep Plane for everything else while your team tries the workflow. Move the rest when you’re ready."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Put your agents where the customer and the work are.",
      "description": "Self-host the Community edition for free, or start a 14-day trial of Helpin Cloud with no card."
    },
    video: { seconds: 38, summary: 'Both are open source. Customer requests fly in from email and chat and miss a tracker that starts at the work item; in Helpin every request lands in one inbox, becomes a task with the customer attached, and the reply flies back.' },
    sources: [
      {
        "label": "Plane pricing",
        "url": "https://plane.so/pricing"
      },
      {
        "label": "Plane billing and plans",
        "url": "https://docs.plane.so/workspaces-and-users/billing-and-plans"
      },
      {
        "label": "Plane AI credits",
        "url": "https://docs.plane.so/ai/plane-ai-credits"
      },
      {
        "label": "Plane self-hosted editions",
        "url": "https://developers.plane.so/self-hosting/editions-and-versions"
      },
      {
        "label": "Plane self-hosting 101",
        "url": "https://developers.plane.so/self-hosting/self-hosting-101"
      },
      {
        "label": "Plane license",
        "url": "https://github.com/makeplane/plane/blob/preview/LICENSE.txt"
      },
      {
        "label": "Plane Customers",
        "url": "https://docs.plane.so/customers"
      },
      {
        "label": "Plane home (Desk coming soon)",
        "url": "https://plane.so/"
      },
      {
        "label": "Plane and Cursor",
        "url": "https://docs.plane.so/integrations/cursor"
      },
      {
        "label": "Plane MCP server",
        "url": "https://developers.plane.so/dev-tools/mcp-server"
      },
      {
        "label": "Plane importers",
        "url": "https://docs.plane.so/importers/overview"
      },
      {
        "label": "Plane export",
        "url": "https://docs.plane.so/core-concepts/export"
      },
      {
        "label": "Plane compliance",
        "url": "https://plane.so/blog/plane-wins-all-top-compliance-certifications"
      },
      {
        "label": "Plane air-gapped requirements",
        "url": "https://developers.plane.so/self-hosting/methods/airgapped-requirements"
      },
      {
        "label": "Plane open source",
        "url": "https://plane.so/open-source"
      },
      {
        "label": "Agent capabilities",
        "url": "https://plane.so/blog/agents-are-now-live-in-plane"
      }
    ],
  },
  {
    slug: 'jira',
    name: 'Jira',
    group: 'Project management',
    category: 'Project management',
    cardLine: "A fast workspace for agent delivery, support, docs and CRM without per-user pricing.",
    checked: "2026-10-03",
    seo: {
      "title": "Open-Source Jira Alternative: Helpin vs Jira",
      "description": "Compare Helpin vs Jira: fast work, AI agents, shared customer context and no per-seat fees. Explore support, projects, docs, CRM and self-hosting."
    },
    hero: {
      "lede": "Turn customer needs into work AI agents can plan, code and review. Helpin puts projects, support, CRM, meetings and docs in one fast workspace, so your team can see the context, approve the changes and keep work moving."
    },
    glance: [
      {
        "label": "Customer support",
        "competitor": "A separate product, priced per agent",
        "helpin": "Chat, email, help center and AI agents built in"
      },
      {
        "label": "Self-hosting",
        "competitor": "Data Center closed to new customers",
        "helpin": "Open source: Cloud or your servers"
      },
      {
        "label": "AI",
        "competitor": "Rovo credits per user, overage billed from December",
        "helpin": "AI usage included in the workspace price"
      }
    ],
    summary: {
      "title": "Spend more of the day moving work forward.",
      "lede": "Helpin brings structured planning and AI delivery into the same workspace as your customers. Check the exact Atlassian features your organization needs.",
      "competitor": [
        "You require Jira’s multi-site administration, capacity planning or sandboxes.",
        "Your workflow depends on specific Atlassian apps or existing migration tools."
      ],
      "helpin": [
        "You want a fast workspace with objectives, roadmaps, epics, sprints and dependencies.",
        "You want agents to investigate, plan, code, review and update docs.",
        "You want support conversations, deals and meeting follow-ups built into the same product.",
        "You want unlimited teammates, an included AI allowance and open-source self-hosting."
      ]
    },
    reasons: [
      {
        "title": "Invite the whole team without seat fees.",
        "icon": "billing",
        "body": "Helpin charges per workspace with unlimited teammates and an included AI allowance. Invite support, engineering and sales without buying another seat. Extra AI usage is metered only if you enable it on a paid plan.",
        "helpinLane": [
          "Workspace price",
          "Unlimited teammates",
          "AI allowance"
        ]
      },
      {
        "title": "Give agents the context to build the right thing.",
        "icon": "loop",
        "body": "A task can carry the original conversation, account history and linked docs. Planning and coding agents can use that context, plus your repository and connected tools, to investigate and prepare a fix. Your team sees the work and reviews the change.",
        "helpinLane": [
          "Report",
          "Agent work",
          "Review",
          "Follow-up"
        ]
      },
      {
        "title": "Keep docs and customer follow-ups moving.",
        "icon": "docs",
        "body": "Quill can update internal guides and your help center after product changes, including fresh UI captures. Once a release is confirmed, configured automation can notify affected customers under your approval rules. The task, guide and conversation stay connected.",
        "helpinLane": [
          "Reviewed change",
          "Docs update",
          "Customer follow-up"
        ]
      },
      {
        "title": "Build agents around the way you work.",
        "icon": "agents",
        "body": "Describe a job in plain language to build a custom agent or flow. Choose its tools, instructions, skills and approval rules. Agents can use connected services for account details, code and logs, so they can investigate before taking the next step.",
        "helpinLane": [
          "Your instructions",
          "Context and tools",
          "Action"
        ]
      }
    ],
    tableLede: "Compare what your team can do in each product. Plan requirements and connected tools are shown where they matter.",
    table: [
      {
        "group": "Pricing",
        "rows": [
          {
            "label": "Price model",
            "helpin": "Per workspace",
            "competitor": "Per user: Standard $9.05, Premium $18.30 a month billed monthly, for up to 100 users"
          },
          {
            "label": "Free plan",
            "helpin": "Self-hosted Community edition",
            "competitor": "Up to 10 users"
          },
          {
            "label": "AI",
            "helpin": "AI usage allowance included in every Cloud plan",
            "competitor": "Rovo credits per user; extra usage billed from December 3, 2026"
          },
          {
            "label": "Support inbox",
            "helpin": "Included",
            "competitor": "Service Collection: free for 3 agents, then $25 an agent a month for up to 15"
          },
          {
            "label": "Open source and self-hosting",
            "helpin": "Free Community edition (AGPL-3.0)",
            "competitor": "Data Center closed to new customers",
            "helpinStatus": "yes",
            "competitorStatus": "no"
          }
        ]
      },
      {
        "group": "Planning",
        "rows": [
          {
            "label": "Roadmaps and sprints",
            "helpin": true,
            "competitor": "Boards and sprints; advanced planning on Premium",
            "competitorStatus": "yes"
          },
          {
            "label": "GitHub and GitLab",
            "helpin": "PR and MR linking",
            "competitor": "GitHub, GitLab and Bitbucket",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Code delivery from a task",
            "helpin": "Built-in planning, coding and review; GitHub PRs and GitLab MRs",
            "competitor": "Rovo, Jira Coding Agent, Claude, Cursor and GitHub Copilot",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          },
          {
            "label": "Integrations and developer tools",
            "helpin": "MCP connections and web SDKs",
            "competitor": "More than 4,000 Marketplace apps"
          },
          {
            "label": "Native mobile apps",
            "helpin": false,
            "competitor": "iOS and Android",
            "competitorStatus": "yes"
          }
        ]
      },
      {
        "group": "Customers",
        "rows": [
          {
            "label": "Support inbox and live chat",
            "helpin": "Web chat and email",
            "competitor": "Service Collection, priced per agent",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "Help center",
            "helpin": "AI answers, own domain, API reference, agent-maintained docs",
            "competitor": "Knowledge base with Service Collection and Confluence",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "CRM with deals",
            "helpin": "Contacts, companies, deals and AI follow-ups",
            "competitor": false,
            "helpinStatus": "yes"
          },
          {
            "label": "Meeting notes",
            "helpin": "Meet, Zoom, Teams and Webex",
            "competitor": "Loom Business + AI, a separate product",
            "helpinStatus": "yes",
            "competitorStatus": "partial"
          },
          {
            "label": "MCP server",
            "helpin": "Read context, update work and start agents",
            "competitor": "Rovo MCP Server",
            "helpinStatus": "yes",
            "competitorStatus": "yes"
          }
        ]
      }
    ],
    strengths: [
      {
        "icon": "enterprise",
        "title": "Large program requirements.",
        "body": "Jira Premium includes capacity planning and sandboxes; Enterprise supports multiple sites. Check these specific needs against your workflow."
      },
      {
        "icon": "ecosystem",
        "title": "Atlassian-specific apps.",
        "body": "Review any Marketplace apps, Confluence or Bitbucket workflows your organization depends on. Helpin’s agents can use connected tools through MCP."
      },
      {
        "icon": "mobile",
        "title": "Native mobile apps.",
        "body": "Jira offers native iOS and Android apps. Helpin currently provides a web workspace."
      },
      {
        "icon": "import",
        "title": "Historical issue migration.",
        "body": "Jira provides import tools for several products. Helpin does not have a Jira importer yet; MCP lets agents use Jira while you run both."
      }
    ],
    calculator: {
      "seatsLabel": "People on the team",
      "seatsUnit": [
        "person",
        "people"
      ],
      "seats": 20,
      "plans": [
        {
          "name": "Standard",
          "annual": 7.54,
          "monthly": 9.05,
          "annualTiers": [
            { "maxSeats": 10, "price": 900 },
            { "maxSeats": 15, "price": 1350 },
            { "maxSeats": 25, "price": 2250 },
            { "maxSeats": 50, "price": 4550 },
            { "maxSeats": 100, "price": 9050 }
          ]
        },
        {
          "name": "Premium",
          "annual": 15.25,
          "monthly": 18.3,
          "annualTiers": [
            { "maxSeats": 10, "price": 1850 },
            { "maxSeats": 15, "price": 2750 },
            { "maxSeats": 25, "price": 4600 },
            { "maxSeats": 50, "price": 9150 },
            { "maxSeats": 100, "price": 18300 }
          ]
        }
      ],
      "plan": 0,
      "helpinPlan": "growth",
      "notes": [
        "Jira annual plans use team-size bands; monthly plans charge per user. This estimate uses the current published bands. Atlassian changes its list prices on October 13, 2026.",
        "Jira doesn’t include a support inbox. Service Collection is free for 3 agents, then $25 an agent a month for up to 15; Loom and Confluence are priced separately too."
      ]
    },
    switching: {
      title: 'Moving from Jira.',
      lede: 'Start with one project or support inbox, then move more work when your team is ready.',
      take: ['Work items, as CSV, Excel or XML', 'Projects and sprints you want to plan in Helpin', 'Confluence pages you want to keep as docs'],
      setUp: ['GitHub or GitLab connection', 'Roadmap, sprints and objectives', 'Support inbox and help center', 'Jira connection for agents, through MCP (beta)'],
      steps: [
        { title: 'Keep Jira, connect it through MCP', status: 'Beta', body: 'Helpin’s agents can use Jira’s tools through Atlassian’s Rovo MCP Server, so engineering can stay in Jira while support runs in Helpin. Your Atlassian admin may need to allow Helpin’s domain.' },
        { title: 'Plan new work in Helpin Projects', status: 'Available now', body: 'Roadmaps, sprints and objectives, with the customer conversation attached to each task.' },
        { title: 'Import Jira work items', status: 'Not yet', body: 'There is no Jira importer yet. Jira exports work items as CSV, Excel or XML, and Helpin imports projects from Shortcut today.' },
      ],
    },
    faqs: [
      [
        "Is Helpin a Jira alternative?",
        "Yes. Helpin provides objectives, roadmaps, epics, sprints and tasks in a fast workspace where agents can plan, code and review. Support, CRM, docs and meetings are built in, with unlimited teammates and open-source self-hosting."
      ],
      [
        "Can I still self-host Jira?",
        "Atlassian stopped selling Data Center to new customers on March 30, 2026. Existing customers can buy until March 30, 2028, and Data Center products become read-only on March 28, 2029. Helpin’s Community edition is open source and runs on your own servers."
      ],
      [
        "Does Jira include a help desk?",
        "No. Atlassian points help-desk teams to Jira Service Management, now part of its Service Collection and priced per agent, with a free plan for 3 agents. Helpin includes the support inbox, chat widget and help center."
      ],
      [
        "How does Jira bill for AI?",
        "Paid Jira plans include Rovo credits per user each month: 25 on Standard, 70 on Premium and 150 on Enterprise. From December 3, 2026, extra usage is billed at $0.01 a credit, and that’s on by default. Rovo Dev is priced separately at $20 per developer a month."
      ],
      [
        "Can Helpin import from Jira?",
        "Not yet. Jira exports work items as CSV, Excel or XML, and Helpin’s agents can use Jira’s tools through MCP while you move."
      ],
      [
        "How is AI billed in Helpin?",
        "Every Cloud plan includes a monthly AI usage allowance, measured in tokens at published rates. On an active paid plan you can turn on metered overage if you need more. Self-hosted installs use your own AI provider and pay it directly."
      ],
      [
        "Is there a free trial?",
        "Yes. The 14-day trial runs on the Growth plan, needs no card, and includes $140 of AI usage."
      ],
      [
        "Can we self-host Helpin?",
        "Yes. The Community edition is free and open source under AGPL-3.0, and runs with Docker Compose. It includes support, projects, CRM, docs and AI agents, including coding and review."
      ],
      [
        "Will you help us switch?",
        "Yes. Book a call and we’ll plan the move with you: what to set up first, how to run both tools side by side, and when to cut over."
      ]
    ],
    closing: {
      "title": "Give agents the work. Keep your team in control.",
      "description": "Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free."
    },
    video: { seconds: 37, summary: 'A customer’s request sinks into the backlog while support, feedback and meeting notes live in separate Atlassian products. Helpin brings it back up with the customer attached, from the task to the pull request and the reply.' },
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
