// Long-form copy for each comparison article. Competitor facts come from the research
// recorded in compare-data.ts (`sources`, `checked`); Helpin facts must match the product
// and /pricing. Shared Helpin paragraphs keep every page describing Helpin the same way.
import type { ProductPreviewName } from '../_components/product-previews/ProductPreview';

type ArticleFeature = {
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
  "inbox": "Helpin gives your team a fast shared inbox for chat and email. Echo, the support agent, can answer from your docs and check connected tools for account details, code and logs. It can follow up after a reply or bring in a teammate. Customer history, internal notes and linked tasks stay with the conversation.",
  "ai": "Ask Helpin AI to investigate an issue and get the work moving. Scribe plans the task, Forge writes and tests the code, Lens reviews it, and Quill updates the docs. Build custom agents and flows in plain language, with your choice of tools, skills and approval rules. Start work in chat or through configured automations.",
  "knowledge": "Keep your help center and internal docs spaces current as your product changes. Quill can turn unanswered questions and released changes into article updates, with fresh screenshots and browser recordings. Your team reviews and publishes them. Customers can search your help center or ask AI for an answer with article citations.",
  "work": "A customer’s bug report can become a task with the conversation attached. AI agents can investigate, plan, write code and open a GitHub pull request or GitLab merge request for review. After a confirmed release, configured automation can update affected customers under your approval rules. Objectives, roadmaps, epics and sprints keep the work in view.",
  "planning": "Plan in a fast workspace with objectives, roadmaps, epics, tasks, stories and sprints. Use triage, dependencies, saved views and reports to keep priorities clear. AI agents can break down a request, take on the code work and prepare it for review. Your team can see the customer need, the plan and the progress together.",
  "crm": "Keep deals, contacts and companies beside their conversations, meetings and open work. Beacon, the sales agent, can identify buying intent, suggest the next step and carry out approved follow-ups. Record meetings on Google Meet, Zoom, Teams or Webex, then turn decisions into assigned tasks. Your team can see what was promised and what still needs doing.",
  "developers": "Connect Claude Code, Codex, Cursor, Hermes and other compatible AI tools through Helpin’s MCP server. They can find context, update tasks and docs, and start Helpin agent runs with the access you allow. Helpin agents can also use external MCP tools. GitHub, GitLab and the web SDKs connect your code and product to the same workflow.",
  "pricing": "Helpin charges per workspace, with unlimited teammates. Starter is $79 and Growth $239 a month billed annually, or $99 and $299 billed monthly. Every Cloud plan includes an AI usage allowance. Extra usage is metered only when you enable it on a paid plan. Self-hosting has no Helpin license fee; you pay for your infrastructure and AI provider.",
  "hosting": "Run Helpin Cloud or self-host the open-source Community edition with Docker Compose. Support, projects, CRM, docs and AI agents are included under AGPL-3.0. When you self-host, you choose the infrastructure, AI providers and external services your workspace uses.",
  "onboarding": "Try Helpin Growth for 14 days with no card and $140 of AI usage. Start with one support address or a real project, invite your team, and give the agents a job. We can help you plan the move around the data and tools you already use."
};

export const ARTICLES: Record<string, Article> = {
  "intercom": {
    "intro": [
      "A customer asks a question. Sometimes the answer is in a guide. Sometimes the product needs fixing. Helpin’s AI agents can handle both kinds of work, with your team in control of what gets approved.",
      "Intercom brings Fin, a shared inbox and multiple support channels together. Helpin brings support, development, sales, meetings and docs into one fast workspace, so your agents and team can work from the same customer history.",
      "You pay per workspace, with unlimited teammates and an included AI allowance. There is no separate fee each time AI resolves a conversation."
    ],
    "difference": [
      {
        "lead": "The answer can lead straight to a fix.",
        "text": "Echo can investigate the question, coding agents can work on the bug, and Quill can update the guide. Your team reviews the work in one workspace."
      },
      {
        "lead": "Intercom combines Fin with a multichannel inbox.",
        "text": "It also connects to external data and runs support procedures. Engineering and sales pipelines generally connect through other products."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Inbox and channels",
        "competitor": "Intercom’s shared inbox covers Messenger chat and email on every plan, with phone, WhatsApp, SMS, social channels, Slack, Discord and Microsoft Teams available, several of them billed by usage. Workflows, multiple team inboxes and round-robin assignment arrive on the Advanced plan.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin keeps chat and email support close to the work needed to solve the problem. Intercom adds phone and social channels.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Fin answers questions using content and connected data, and can run multistep procedures such as account lookups and other support actions. Intercom charges $0.99 per Fin outcome on top of seats. Its Copilot assists human teammates with replies.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin carries AI work across support, code, review and docs, with your team deciding what needs approval.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Intercom includes a public help center on every plan, with multilingual and private help centers on Advanced and multibrand help centers on Expert. Fin uses your articles and other content to answer.",
        "helpin": HELPIN.knowledge,
        "verdict": "With Helpin, keeping answers accurate includes updating the guide and capturing the current UI after the product changes."
      },
      {
        "id": "question-to-fix",
        "title": "From question to fix",
        "competitor": "Intercom can send customer issues to project tools such as Jira. Integrations connect the conversation to engineering work, while planning and code delivery happen in those connected tools.",
        "helpin": HELPIN.work,
        "verdict": "Helpin gives the support team, developers and AI agents one place to take a customer issue through to delivery.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "Customer records, CRM and meetings",
        "competitor": "Intercom keeps contacts, companies and leads, and connects to CRMs such as Salesforce and HubSpot for deals. It doesn’t record meetings.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin turns account history into sales follow-ups and assigned work, with deals and meeting notes built in.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "Developers and integrations",
        "competitor": "Intercom offers APIs, webhooks, native mobile SDKs and an app marketplace. Its hosted MCP server gives AI tools access to workspace data, and Fin can connect to external MCP tools.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin connects your agents to the context and actions across your business. Check Intercom’s native SDKs if mobile chat is essential."
      }
    ],
    "pricing": "Intercom charges per seat: $29, $85 or $132 a month billed annually for Essential, Advanced and Expert. Fin adds $0.99 per outcome, and channels such as phone, SMS and WhatsApp, and add-ons such as Copilot and Proactive Support, are billed separately.",
    "hosting": "Intercom is a hosted service, and there’s no way to run it on your own infrastructure.",
    "onboarding": "Intercom offers a 14-day trial with no card. It has no one-click importer: moving historical data into Intercom is a scripted, API-based migration, and conversations can be exported as CSV, through the API or to cloud storage."
  },
  "zendesk": {
    "intro": [
      "Closing a ticket is useful. Fixing the problem behind it is better. Helpin gives AI agents the customer conversation, the account history and the tools to investigate, write code and prepare a change for review.",
      "Zendesk brings together ticketing, messaging, voice and AI service tools. Helpin connects chat and email support directly to projects, CRM, meetings and docs in a fast workspace your whole team can use.",
      "Bring support and development into the same workflow without buying a seat for every teammate. Cloud plans include AI usage, with no per-resolution fee."
    ],
    "difference": [
      {
        "lead": "The ticket and the work share the same context.",
        "text": "Helpin’s agents can move from support to code, docs and customer follow-up, with the account history close at hand."
      },
      {
        "lead": "Zendesk brings service channels together.",
        "text": "It includes ticketing, voice and AI actions. Product development and sales workflows connect through other tools."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Ticketing and channels",
        "competitor": "Zendesk supports email, chat, messaging and voice. Its contact center provides phone menus, and Suite Professional includes routing by agent skills. Plans also offer ticket rules and response-time policies.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin connects each chat or email to the account and product work. Check Zendesk’s voice tools if you run a phone queue.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Zendesk’s AI agents can answer questions and carry out actions in connected systems. Suite plans include an automated-resolution allowance; additional resolutions are billed separately. Copilot assists teammates and is listed at $50 per agent per month as an add-on.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin’s agents can work on the cause of a ticket, including code and docs, with usage included in every Cloud plan.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Zendesk includes a knowledge base and help center on Suite plans, which its AI agents draw on to answer customers.",
        "helpin": HELPIN.knowledge,
        "verdict": "Helpin connects knowledge gaps to article updates, product changes and fresh UI captures, for customers and your own team."
      },
      {
        "id": "question-to-fix",
        "title": "From ticket to fix",
        "competitor": "Zendesk manages the ticket and connects to tools like Jira for engineering work, so the fix and its status live in another system.",
        "helpin": HELPIN.work,
        "verdict": "Helpin keeps the customer report, agent work and reviewed fix connected, so fewer details need to be passed between teams.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "CRM and meetings",
        "competitor": "Zendesk connects service records to external CRMs. Zendesk Sell, its separate sales product, is scheduled to retire on August 31, 2027. Sales pipelines and recorded meeting workflows need separate tools.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin includes the sales pipeline and meeting context that help agents choose and carry out the next step.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "Developers and integrations",
        "competitor": "Zendesk offers APIs, mobile SDKs and marketplace integrations. Its MCP client is generally available, so AI workflows can fetch data and take actions in connected tools. Administrators choose which tools those workflows can use.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin brings external tools into agent workflows and gives compatible agents access to tasks, conversations and docs."
      }
    ],
    "pricing": "Zendesk charges per agent: Suite Team is $55 and Suite Professional $115 a month billed annually, with Enterprise priced on request. AI agent resolutions beyond the included allowance, and Copilot at $50 per agent, are billed separately.",
    "hosting": "Zendesk is a hosted service, and there’s no way to run it on your own infrastructure.",
    "onboarding": "Zendesk offers a 14-day trial with no card, running on Suite Professional. Ticket and user exports are off by default and must be enabled by Zendesk on request, and full JSON exports need higher plans."
  },
  "help-scout": {
    "intro": [
      "A simple inbox should not leave your team doing all the follow-up work. In Helpin, AI agents can answer a customer, check their account, create a task and help get the underlying problem fixed.",
      "Help Scout offers a shared inbox, Docs and AI answers. Helpin keeps chat and email just as central, while giving your team a fast workspace for projects, deals, meetings and the docs that keep answers accurate.",
      "Invite the people who need the context without adding seat fees. Every Cloud plan includes an AI allowance, and Helpin can import your Help Scout Docs articles."
    ],
    "difference": [
      {
        "lead": "Keep the inbox simple and automate what follows.",
        "text": "Helpin’s agents can investigate issues, do product work and update docs while your team stays in one fast workspace."
      },
      {
        "lead": "Help Scout centers on customer conversations.",
        "text": "Its inbox, Docs and AI answers cover support, with integrations for projects and sales."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Inbox and channels",
        "competitor": "Help Scout’s inbox covers email, live chat through Beacon, WhatsApp, Instagram, Messenger and SMS, with phone through integrations such as Aircall. Workflows, SLA policies and inbox limits scale with the plan: Standard includes two inboxes and Plus five.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin gives chat and email teams a fast inbox with agents that can investigate and follow up. Help Scout adds social channels.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Help Scout sells AI Answers at $0.75 per resolution and presents its customer-facing agent as Compass. It answers from Docs and other supplied content. Its agent page lists app connections and email support as coming soon; separate AI Assist, Drafts and Summarize features help teammates in the inbox.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin can take a support request into planning, code and review, with the same customer context available throughout.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Help Scout Docs publishes knowledge base sites: two on Standard and three on Plus, with more available as an add-on. AI Answers draws on them.",
        "helpin": HELPIN.knowledge,
        "verdict": "Bring your Help Scout articles into Helpin, then use Quill to keep guides and internal docs current as you ship."
      },
      {
        "id": "question-to-fix",
        "title": "From question to fix",
        "competitor": "Help Scout connects to Jira on the Plus plan and above, so engineering work lives in another tool.",
        "helpin": HELPIN.work,
        "verdict": "Helpin connects the conversation to agents that can work on the fix, then supports customer follow-up after release.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "Customer records, CRM and meetings",
        "competitor": "Help Scout keeps customer properties, company profiles and, on higher plans, custom fields, and integrates with HubSpot and Salesforce on Plus. It doesn’t include a deals pipeline or meeting notes.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin helps your team act on sales intent and meeting decisions, with a built-in deal pipeline and owned follow-ups.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "Developers and integrations",
        "competitor": "Help Scout provides APIs, webhooks, integrations and Beacon SDKs for iOS and Android. Its MCP connection can search conversations and pull information. Current connections are read-only: external agents cannot send replies, change tags or edit Docs through it.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin’s MCP tools support permitted changes as well as reading, so your external agents can move the work forward."
      }
    ],
    "pricing": "Help Scout charges per user: Standard is $25 and Plus $45 a month billed annually, with Pro from ten users. A free plan covers up to five users and 100 contacts a month, and AI Answers adds $0.75 per resolution.",
    "hosting": "Help Scout is a hosted service, and there’s no way to run it on your own infrastructure.",
    "onboarding": "Help Scout offers a 15-day trial with no card and a 30-day money-back guarantee on the first payment. Its built-in importer brings conversations, customers and tags from more than 30 tools. Helpin imports Help Scout Docs today; conversation history isn’t imported yet."
  },
  "chatwoot": {
    "intro": [
      "If you are choosing open source, look at what you can actually run. Helpin’s free Community edition includes support, projects, CRM, docs and AI agents that can plan, code, review and follow up.",
      "Chatwoot offers a shared inbox across chat, email and messaging channels. Helpin puts chat and email beside the product work and customer history in one fast workspace, so agents can act on more than the conversation alone.",
      "Self-host Helpin with your own AI provider, or choose Cloud with workspace pricing and an included AI allowance. You do not need an extra AI license when you self-host."
    ],
    "difference": [
      {
        "lead": "Self-host the agents and the work they do.",
        "text": "Helpin includes support, coding, review, docs and sales agents in its free Community edition. Bring your own AI provider."
      },
      {
        "lead": "Chatwoot connects more messaging channels.",
        "text": "Its MIT-licensed inbox can be extended with bots and a CLI. Captain requires a paid self-hosted plan."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Inbox and channels",
        "competitor": "Chatwoot’s inbox covers website chat, email, WhatsApp, Facebook, Instagram, TikTok, Telegram, LINE and SMS, with voice calls in beta. On Cloud, automation rules, teams and voice need the Business plan.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin connects chat and email to product work and sales. Chatwoot also covers phone and social messaging.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Captain answers customers, assists teammates and supports custom support scenarios. Cloud plans include credits, with extra usage available to buy. Self-hosted Captain requires a paid plan and your own AI provider. Chatwoot also supports custom inbox bots.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin includes its support, coding, review and docs agents in the free self-hosted edition. Your AI provider bills for usage.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Chatwoot includes a help center with a documentation-style layout, search and per-locale branding, which Captain can use to answer.",
        "helpin": HELPIN.knowledge,
        "verdict": "Helpin’s docs workflow extends to internal guides, release-driven updates and browser captures of the actual product."
      },
      {
        "id": "question-to-fix",
        "title": "From conversation to fix",
        "competitor": "Chatwoot links conversations to issues through its Linear integration, so planning and engineering happen in another tool.",
        "helpin": HELPIN.work,
        "verdict": "Helpin has the planning, coding and review workflow built in, with the original conversation connected to the task.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "Customer records, CRM and meetings",
        "competitor": "Chatwoot keeps contacts, custom attributes, segments and companies. It doesn’t include a deals pipeline or meeting notes.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin turns customer context into sales action with deals, meeting notes and Beacon’s follow-ups in the same workspace.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "Developers and integrations",
        "competitor": "Chatwoot offers APIs, webhooks, mobile SDKs and an official CLI for coding agents. Through the CLI, external agents can inspect conversations and prepare support actions for approval. It works with both Cloud and self-hosted installations.",
        "helpin": HELPIN.developers,
        "verdict": "Both work with external agents. Helpin also provides built-in agents and an MCP server across support, projects, docs and CRM."
      }
    ],
    "pricing": "Chatwoot Cloud charges per agent: $19, $39 or $99 a month, with Captain credits included and extra credits at $20 per 1,000. Self-hosted, the Community edition is free, and the Premium plan that adds Captain costs $19 per agent.",
    "hosting": "Chatwoot is open source and can be deployed with Docker, Kubernetes, a Linux installer or cloud marketplace images. Features in its enterprise edition need a paid subscription for production use.",
    "onboarding": "Chatwoot offers a 15-day Cloud trial and a free Hacker plan for up to two agents, and can import from Intercom and Freshdesk. There’s no Chatwoot importer in Helpin yet; both products have APIs if you need to move records yourself."
  },
  "linear": {
    "intro": [
      "Keep the speed you expect from a modern project tool. Add AI agents that can work from the customer’s first message through planning, code review, docs updates and follow-up.",
      "Linear brings issues, projects, cycles and agents together. Helpin gives your team a fast planning workspace too, with a shared support inbox, CRM, meeting notes and help center built in. The people doing the work can see why it matters.",
      "You can use Helpin’s built-in agents, create your own, or connect compatible external agents through MCP. One workspace price includes unlimited teammates, and you can self-host the open-source edition."
    ],
    "difference": [
      {
        "lead": "Give agents the job and the reason behind it.",
        "text": "Helpin connects planning and code to live support conversations, deals, meetings and docs in the same product."
      },
      {
        "lead": "Linear connects engineering to customer requests.",
        "text": "It links feedback and account details to issues, with support conversations handled in connected tools."
      }
    ],
    "features": [
      {
        "id": "planning",
        "title": "Roadmaps, sprints and triage",
        "competitor": "Linear organizes work into issues, projects, cycles and initiatives, with triage, views and reporting. Insights dashboards are available on Business. Its interface emphasizes quick navigation and keyboard shortcuts.",
        "helpin": HELPIN.planning,
        "verdict": "Helpin combines fast planning with agents that act on the work and built-in access to the customers asking for it.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "coding-agents",
        "title": "AI agents that carry out work",
        "competitor": "Linear Agent can write code in cloud coding sessions using Claude Code or Codex, paid for with prepaid AI credits, and you can assign issues to third-party agents such as Cursor, Codex, GitHub Copilot and Devin.",
        "helpin": HELPIN.ai,
        "verdict": "Both support coding agents. Helpin also connects planning, review, docs upkeep and customer follow-up in the same workspace.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "customer-requests",
        "title": "Customer requests and support",
        "competitor": "Linear links customer requests to issues and keeps account details such as revenue and customer tier. Connections to tools such as Intercom and Zendesk bring in support context. Customer-facing chat, email and help-center publishing happen in separate tools.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin keeps the actual conversation and the development work together, so agents and people can act on the same history.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "crm",
        "title": "Customer records and meetings",
        "competitor": "Linear tracks customer requests and attributes, and can turn Gong call transcripts into requests on Enterprise. It has no CRM or meeting notes of its own.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin gives your team and agents a deal pipeline, meeting decisions and follow-ups alongside the work customers are waiting for.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "Git, API and integrations",
        "competitor": "Linear connects to GitHub and GitLab for pull request linking, and offers a GraphQL API, webhooks, a hosted MCP server and native iOS and Android apps.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin supports your existing AI tools and connects agents to external systems, including Linear, through MCP."
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Linear includes documents for specs and project updates, but no public help center for customers.",
        "helpin": HELPIN.knowledge,
        "verdict": "Helpin covers the full docs workflow, from internal decisions to current customer guides with screenshots and browser recordings."
      }
    ],
    "pricing": "Linear charges per user: Basic is $10 and Business $16 a month billed annually, with a free plan for up to 250 issues. Coding sessions use prepaid AI credits, the integrations with support tools need Business, and your support tool is priced separately.",
    "hosting": "Linear is a hosted service, and there’s no way to run it on your own infrastructure.",
    "onboarding": "Linear provides importers for tools including Jira, GitHub Issues, Asana and Shortcut. Helpin does not import Linear history yet. You can start a new project in Helpin and connect Linear through MCP while you decide what to move."
  },
  "plane": {
    "intro": [
      "Your agents need more than a task description. They need the customer’s conversation, the account history, the relevant docs and the code. Helpin brings that context into one fast workspace where agents can do the work.",
      "Plane combines project planning, a wiki and AI workflows. Helpin combines objectives, roadmaps, sprints and epics with chat and email support, CRM, meetings and agents for planning, coding, review and docs.",
      "Both offer self-hosting. Helpin includes its AI agents and product features in the free open-source edition, while Cloud uses one workspace price with unlimited teammates."
    ],
    "difference": [
      {
        "lead": "Plan, build and follow up in one workspace.",
        "text": "Helpin’s agents can work across customer support, projects, code, docs and sales. Those capabilities are included when you self-host."
      },
      {
        "lead": "Plane connects projects, knowledge and AI.",
        "text": "Its paid commercial edition adds AI and agents. Its customer help desk is listed as coming soon."
      }
    ],
    "features": [
      {
        "id": "planning",
        "title": "Roadmaps, cycles and sprints",
        "competitor": "Plane includes work items, cycles, modules, epics and multiple layouts. Paid plans add features such as initiatives, dashboards and time tracking. It also provides workflows, approvals and recurring work items on higher plans.",
        "helpin": HELPIN.planning,
        "verdict": "Helpin gives teams structured planning and fast day-to-day work, with AI agents and customer conversations connected to delivery.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Plane AI can answer questions, create work items and run custom agents from mentions, assignments or schedules. Cloud plans use personal AI allowances and a separate shared budget for agent runs. Its Cursor integration can turn assigned work into a GitHub pull request.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin’s agents cover the customer question, the code, the review and the guide update. They are included when you self-host.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "customer-requests",
        "title": "Customer requests and support",
        "competitor": "Plane collects requests through Intake, with public forms and an intake email address on Business, and links them to customer profiles with Customers on Business. Its Desk help desk is listed as coming soon, and there’s no live chat.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin provides the live conversation as well as the task, so you can answer, investigate and follow up in one place.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "crm",
        "title": "Customer records and meetings",
        "competitor": "Plane’s Customers feature on Business gives each customer a profile with fields such as stage, contract status and revenue, linked to their requests. It has no deals, pipelines or meeting notetaker; Plane AI can search Granola meeting notes through a connector.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin adds sales pipelines and recorded meeting context, with agents that can turn both into action.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "self-hosting",
        "title": "Open source and self-hosting",
        "competitor": "Plane has an AGPL-3.0 Community Edition and a separate Commercial Edition for paid features, including AI. It supports Docker, Kubernetes and an air-gapped Enterprise deployment. Self-hosted AI uses your own provider.",
        "helpin": HELPIN.hosting,
        "verdict": "Helpin includes support, projects, CRM and AI agents without a self-hosted license fee. Compare deployment needs separately."
      },
      {
        "id": "developers",
        "title": "Git, API and integrations",
        "competitor": "Plane connects to GitHub, GitLab, Slack and Sentry on Pro and above, offers a REST API with webhooks, an MIT-licensed MCP server and native mobile apps, and imports from Jira, Linear, Asana and ClickUp.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin connects built-in and external agents to work across the business. Check specific importers if you need a historical migration."
      }
    ],
    "pricing": "Plane charges per seat: Pro is $6 and Business $13 a month billed annually, or $8 and $15 billed monthly. Its free plan covers up to 12 seats. Paid Cloud plans include personal AI allowances and a separate workspace allowance for agent runs. Helpin charges per workspace, so inviting another teammate does not increase the subscription.",
    "hosting": "Plane’s Community Edition is open source under AGPL-3.0 and matches its Free plan. Paid features, including Plane AI, run in the closed-source Commercial Edition with a license key and your own AI provider. An air-gapped edition is available on Enterprise Grid, with a 100-seat minimum.",
    "onboarding": "Plane imports from Jira, Linear, Asana and ClickUp, and exports work items as CSV, Excel or JSON. Helpin doesn’t import from Plane yet, but its agents can use Plane Cloud’s tools through MCP, so you can run both during a move."
  },
  "jira": {
    "intro": [
      "Spend your day moving work forward, with AI agents taking on the investigation, planning, code and review. Helpin keeps that work in a fast workspace alongside the customer conversations that started it.",
      "Jira provides project tracking and AI agents within the Atlassian family. Helpin brings objectives, roadmaps, epics and sprints together with support, CRM, meetings and docs, so teams can work from the same context without assembling several products.",
      "Give agents access to your code and connected tools, choose what needs approval, and keep the progress visible. Helpin charges per workspace with unlimited teammates and offers a free open-source edition you can self-host."
    ],
    "difference": [
      {
        "lead": "Bring the customer and the delivery work together.",
        "text": "Helpin includes agents, planning, support, docs, CRM and meetings in one product, with one workspace price."
      },
      {
        "lead": "Jira connects to the wider Atlassian family.",
        "text": "Support, knowledge and meeting workflows can span Service Collection, Confluence and Loom."
      }
    ],
    "features": [
      {
        "id": "planning",
        "title": "Roadmaps, boards and sprints",
        "competitor": "Jira organizes work into projects, boards and sprints for Scrum and Kanban teams. Premium adds advanced planning, capacity management, approvals and sandboxes; Enterprise adds multiple sites and Atlassian Analytics.",
        "helpin": HELPIN.planning,
        "verdict": "Helpin combines structured planning, a fast workspace and agents that carry out the work with the customer context attached.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "coding-agents",
        "title": "AI agents that carry out work",
        "competitor": "Jira supports Rovo agents and connected coding agents, including tools from Claude, Cursor and GitHub Copilot. Paid plans include Rovo credits; Rovo Dev has separate pricing. Agents can work with issues and connected repositories, with access controlled through the Atlassian setup.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin supports built-in, custom and external agents across support, development, docs and sales, with tools and approvals you choose.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "customer-requests",
        "title": "Customer requests and support",
        "competitor": "Atlassian provides customer support through its separate Service Collection, which includes Jira Service Management and customer service tools. These can connect tickets and conversations to Jira work. Support subscriptions are priced separately from Jira.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin includes the shared inbox and help center alongside projects, so the question and the work share one history.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "crm",
        "title": "Customer records and meetings",
        "competitor": "Atlassian connects to external CRMs for sales pipelines. Meeting recording and AI notes come through Loom, with Confluence and Jira used for the resulting knowledge and work. Rovo can use connected information across these products.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin brings deals, customer conversations and meeting follow-ups into the same product as your roadmap and AI agents.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "hosting",
        "title": "Hosting and open source",
        "competitor": "Jira Cloud is hosted by Atlassian. Data Center, the self-managed edition, closed to new customers on March 30, 2026, and becomes read-only on March 28, 2029. Jira isn’t open source.",
        "helpin": HELPIN.hosting,
        "verdict": "Helpin gives you a free open-source path to self-hosting, including AI agents, projects, support and CRM."
      },
      {
        "id": "developers",
        "title": "Git, API and integrations",
        "competitor": "Jira connects to GitHub, GitLab and Bitbucket, offers a REST API and webhooks, native iOS and Android apps, the official Rovo MCP Server and more than 4,000 Marketplace apps.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin connects to your code and AI tools, and its agents can use Jira through MCP while both products run together."
      }
    ],
    "pricing": "Jira charges per user. Billed monthly, Standard is $9.05 and Premium $18.30 a user for up to 100 users; annual plans are priced by user tier, such as $9,050 a year for 100 users on Standard. The free plan covers up to 10 users, and Atlassian raises these prices on October 13, 2026. The help desk, product feedback, docs and meeting notes are priced separately.",
    "hosting": "Jira Cloud is hosted by Atlassian. New customers can no longer buy Data Center, the self-managed edition, and Data Center products become read-only on March 28, 2029. Jira isn’t open source.",
    "onboarding": "Jira imports from CSV, Asana, monday, ClickUp, Trello, Linear, GitHub, GitLab and more, and exports work items as CSV, Excel or XML. Helpin doesn’t import from Jira yet, but its agents can use Jira’s tools through MCP, so you can run both during a move."
  },
  "chatbase": {
    "intro": [
      "An AI answer can save a customer a few minutes. An agent that works on the underlying problem can save your whole team a round of handoffs. Helpin connects the conversation to the plan, code, review and docs in one fast workspace.",
      "Chatbase provides customer-facing AI agents, procedures, a helpdesk and connected actions. Helpin covers AI support too, with projects, CRM, meetings and agents that can take on the work after the conversation.",
      "Both include AI usage in their paid plans. Helpin also gives you unlimited teammates and an open-source edition you can run with your own AI provider."
    ],
    "difference": [
      {
        "lead": "Helpin brings the customer and the work together.",
        "text": "Agents can investigate an issue, plan and code a fix, update the guide and follow up after a confirmed release, under your approval rules."
      },
      {
        "lead": "Chatbase builds around customer-facing agents.",
        "text": "Its procedures, helpdesk and integrations support answers, actions and human handoffs across customer channels."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Support inbox and customer context",
        "competitor": "Chatbase includes a helpdesk for live takeover and email follow-up. It brings website, email and social conversations together, with ticket assignment, internal notes and conversation history. Teams can also connect an existing helpdesk.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin connects the support conversation directly to projects, deals and agent work, so more of the next step happens in one place.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Chatbase agents can follow procedures, look up information and take actions through connected systems. Procedures describe the steps in plain language, with testing and human handoff available.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin extends agent work to planning, coding, review and docs, with the customer context available throughout.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Chatbase trains agents on supplied content and can resync sources. Its Agent Page gives customers a place to ask questions using existing documentation. The source articles still need to be maintained.",
        "helpin": HELPIN.knowledge,
        "verdict": "Helpin can maintain the underlying guides as well as answer from them, including fresh screenshots and browser recordings."
      },
      {
        "id": "question-to-fix",
        "title": "From customer report to reviewed fix",
        "competitor": "Chatbase can collect an issue, create a support ticket and use connected actions to move it along. A project tracker and coding workflow handle the engineering work beyond the helpdesk.",
        "helpin": HELPIN.work,
        "verdict": "Helpin has the task, planning, coding and review workflow built in, with the original conversation attached.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "Sales follow-ups and meeting context",
        "competitor": "Chatbase can qualify leads, handle pricing questions and book demos. CRM integrations connect those conversations to systems your sales team already uses.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin gives agents a built-in deal pipeline and meeting decisions, so sales follow-ups and product work use the same account history.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "External agents and connected tools",
        "competitor": "Chatbase’s MCP server lets compatible clients manage agents, knowledge sources, conversations and tickets within their permissions. It also provides APIs and web, voice and mobile SDKs.",
        "helpin": HELPIN.developers,
        "verdict": "Both connect to external AI tools. Helpin gives those agents access to the wider workspace, including tasks, docs and agent runs."
      }
    ],
    "pricing": "Chatbase’s monthly plans are Hobby at $40, Standard at $150 and Pro at $500. They include 2, 3 and 5 members and different message-credit allowances. Yearly plans have a published 20% discount. Extra message credits, additional AI agents and branding removal have separate prices. Helpin’s subscription includes unlimited teammates; compare the AI allowance and the other tools included too.",
    "hosting": "Chatbase is a hosted platform. Its open-source SDKs connect to that service; they do not provide a self-hosted Chatbase workspace.",
    "onboarding": "Chatbase offers a free plan and a seven-day paid-plan trial. When moving to Helpin, start from your existing source content and instructions. There is no direct Chatbase workspace importer, so keep historical records available while you set up and review the new workflow."
  },
  "crisp": {
    "intro": [
      "A shared inbox gives conversations a place to land. Helpin connects them to the work needed to solve the problem, with AI agents that can investigate, plan, write code and prepare the result for review.",
      "Crisp combines a shared inbox, Hugo, workflows and a knowledge base. Helpin brings support into the same fast workspace as projects, deals, meetings and agents for coding, review and docs.",
      "Both use workspace pricing and include AI usage on paid plans. Helpin adds unlimited teammates on every plan and the option to self-host the product and its agents."
    ],
    "difference": [
      {
        "lead": "Helpin connects support to delivery and sales.",
        "text": "The customer’s conversation can become agent work, a reviewed fix, a guide update or an owned follow-up in the same product."
      },
      {
        "lead": "Crisp connects support channels and automation.",
        "text": "Its inbox, Hugo and workflows handle customer conversations, with connected tools extending the actions they can take."
      }
    ],
    "features": [
      {
        "id": "inbox",
        "title": "Support inbox and customer context",
        "competitor": "Crisp brings chat, email and connected messaging channels into a shared inbox. Its support tools include notes, routing, saved replies and follow-up reminders. Mobile apps let teammates reply while away from their desks.",
        "helpin": HELPIN.inbox,
        "verdict": "Helpin keeps the inbox close to product delivery and account work. Crisp also covers social messaging and native mobile access.",
        "preview": {
          "product": "inbox",
          "label": "Helpin’s shared inbox: the customer’s earlier conversation, linked task and AI draft in one view."
        }
      },
      {
        "id": "ai-agents",
        "title": "AI agents that carry out work",
        "competitor": "Hugo answers questions and takes actions through connected services. Crisp also provides visual workflows, task automations and MCP connections for custom tools and data.",
        "helpin": HELPIN.ai,
        "verdict": "Helpin carries agent work into planning, code, review and docs, alongside customer support.",
        "preview": {
          "product": "agents",
          "label": "Helpin’s specialist agents, each with its own tools and approval rules."
        }
      },
      {
        "id": "knowledge",
        "title": "Help center and internal docs",
        "competitor": "Crisp includes a customer knowledge base on Essentials and Plus. Hugo can use help content and other connected knowledge to answer customers, while teammates manage the articles.",
        "helpin": HELPIN.knowledge,
        "verdict": "Helpin’s docs workflow covers internal spaces and public guides, including agent-prepared release updates and fresh UI captures."
      },
      {
        "id": "question-to-fix",
        "title": "From support conversation to reviewed fix",
        "competitor": "Crisp’s task automations can call connected tools to carry out support actions. Engineering planning and code delivery run in the project and development tools you connect to that workflow.",
        "helpin": HELPIN.work,
        "verdict": "Helpin includes the project structure and coding workflow, so your team can review the fix beside the customer evidence.",
        "preview": {
          "product": "projects",
          "label": "Helpin Projects: the task, the customer conversation behind it and the agent’s progress."
        }
      },
      {
        "id": "crm",
        "title": "Sales follow-ups and meeting context",
        "competitor": "Crisp keeps customer profiles, custom data and conversation history. It connects to sales systems such as HubSpot and Pipedrive so teams can use customer data in their support workflow.",
        "helpin": HELPIN.crm,
        "verdict": "Helpin adds a built-in deal pipeline, recorded meetings and agents that can act on the next sales step.",
        "preview": {
          "product": "crm",
          "label": "Helpin CRM: deals with the conversations, meetings and work behind them."
        }
      },
      {
        "id": "developers",
        "title": "External agents and connected tools",
        "competitor": "Crisp offers APIs, webhooks and mobile SDKs. Hugo can call tools on external MCP servers. Its own MCP server currently exposes knowledge search for compatible AI clients.",
        "helpin": HELPIN.developers,
        "verdict": "Helpin’s MCP tools also let external agents update tasks and docs and start agent runs, with the access you allow."
      }
    ],
    "pricing": "Crisp charges per workspace: Mini is $45, Essentials $95 and Plus $295 per month. Plans include 4, 10 and 20 seats respectively; Plus lists additional seats at $10 each per month. Paid plans include Hugo credits, with optional pay-as-you-go. Helpin includes unlimited teammates and an AI allowance, plus projects, CRM and meetings. Neither product charges a standard fee for each resolved conversation.",
    "hosting": "Crisp is a hosted service. Its SDKs and open-source developer tools connect to Crisp Cloud; the full workspace is not offered as a self-hosted product.",
    "onboarding": "Crisp offers a free plan and a 14-day trial. To move into Helpin, bring over your help content and workflow instructions, then start with a support address or a few website pages. There is no direct Crisp conversation importer today."
  }
};

export const HELPIN_ARTICLE = HELPIN;
