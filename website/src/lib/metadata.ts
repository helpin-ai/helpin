import type { Metadata } from 'next';

export const SITE_URL = 'https://helpin.ai';

export type PageSeo = {
  title: string;
  description: string;
  canonicalPath: string;
  imagePath: string;
  imageAlt: string;
};

export const PAGE_SEO = {
  home: {
    title: "AI agents for support, projects, and CRM | Helpin",
    description: "AI agents answer customers, update docs, plan work, and help sales follow up. One open-source workspace for your team. Self-host or use Helpin Cloud.",
    canonicalPath: '/',
    imagePath: '/og/helpin-new-home-green-v5.png',
    imageAlt: 'Helpin · AI agents that do more than answer.',
  },
  product: {
    title: "AI agents and connected team tools | Helpin",
    description: "Explore Helpin for support, projects, CRM, meetings, and docs. See how AI agents help with everyday work and how your team reviews the results.",
    canonicalPath: '/product',
    imagePath: '/og/helpin-product-green-v5.png',
    imageAlt: 'Helpin product overview',
  },
  customerSupport: {
    title: "AI customer support and shared inbox | Helpin",
    description: "The Echo agent answers questions, follows up, and hands over with the findings. Manage chat, email, and team inboxes in Helpin. Cloud or self-hosted.",
    canonicalPath: '/products/customer-support',
    imagePath: '/og/helpin-customer-support-green-v5.png',
    imageAlt: 'Helpin customer support',
  },
  meetings: {
    title: "AI meeting notes and action items | Helpin",
    description: "Capture team calls and customer demos. Get AI notes, decisions, and action items, then use Ask Agent to help turn the discussion into tracked work.",
    canonicalPath: '/products/meetings',
    imagePath: '/og/helpin-meetings-green-v5.png',
    imageAlt: 'Helpin meetings',
  },
  projects: {
    title: "AI project management and coding agents | Helpin",
    description: "Plan tasks, epics, sprints, and roadmaps. AI agents plan, code, and review changes while your team keeps the work and decisions in view.",
    canonicalPath: '/products/projects',
    imagePath: '/og/helpin-projects-green-v5.png',
    imageAlt: 'Helpin projects',
  },
  crm: {
    title: "AI CRM and sales follow-up automation | Helpin",
    description: "Find who needs attention and prepare a relevant follow-up with the Beacon agent. Keep contacts, deals, email, and customer conversations together.",
    canonicalPath: '/products/crm',
    imagePath: '/og/helpin-crm-green-v5.png',
    imageAlt: 'Helpin CRM',
  },
  knowledge: {
    title: "AI knowledge base and help center software | Helpin",
    description: "The Quill agent prepares guide updates from customer questions and product changes. Publish help articles and API docs with your team in control.",
    canonicalPath: '/products/knowledge',
    imagePath: '/og/helpin-knowledge-green-v5.png',
    imageAlt: 'Helpin knowledge',
  },
  aiAgents: {
    title: "AI agents for support, coding, and sales | Helpin",
    description: "Meet the Echo, Scribe, Forge, Quill, and Beacon agents. Automate useful work with selected tools, scheduled runs, and approvals your team controls.",
    canonicalPath: '/products/ai-agents',
    imagePath: '/og/helpin-ai-agents-green-v5.png',
    imageAlt: 'Helpin AI agents',
  },
  developers: {
    title: "Support SDKs and agent integrations | Helpin",
    description: "Embed support, verify customer identity, connect permitted account tools, and start agent work from events. Explore Helpin SDKs, APIs, MCP, and the CLI.",
    canonicalPath: '/developers',
    imagePath: '/og/helpin-developers-green-v5.png',
    imageAlt: 'Build on Helpin',
  },
  selfHosting: {
    title: "Self-hosted AI agents and support software | Helpin",
    description: "Run Helpin support, projects, CRM, docs, meetings, and AI agents on your own servers. Open-source Community edition with your own AI providers.",
    canonicalPath: '/self-hosting',
    imagePath: '/og/helpin-self-hosting-green-v5.png',
    imageAlt: 'Self-host Helpin',
  },
  branding: {
    title: 'Brand kit: logo, colors and typography · Helpin',
    description:
      'Download the Helpin logo, symbols, color palette and typography for articles, integrations and presentations.',
    canonicalPath: '/branding',
    imagePath: '/og/helpin-branding-green-v5.png',
    imageAlt: 'Helpin brand kit',
  },
  pricing: {
    title: 'Helpin pricing · Free to self-host, Cloud with AI included',
    description:
      'Self-host Helpin free, or choose Helpin Cloud from $79/month billed annually, with AI usage included. One price per workspace, no per-seat fees.',
    canonicalPath: '/pricing',
    imagePath: '/og/helpin-pricing-green-v5.png',
    imageAlt: 'Helpin pricing · Your team. Your AI agents. One workspace.',
  },
  compare: {
    title: 'Compare Helpin with Intercom, Zendesk, Linear and more',
    description:
      'Compare Helpin with Intercom, Zendesk, Linear, Jira and more. AI agents, fast work and all your customer context in one workspace, with no per-seat fees.',
    canonicalPath: '/compare',
    imagePath: '/og/helpin-compare-green-v5.png',
    imageAlt: 'Compare Helpin',
  },
  privacy: {
    title: 'Privacy Policy · Helpin',
    description:
      'Learn how Helpin collects, protects, and processes information across the website and connected workspace.',
    canonicalPath: '/privacy',
    imagePath: '/og/helpin-privacy-green-v5.png',
    imageAlt: 'Privacy at Helpin',
  },
  terms: {
    title: 'Terms of Service · Helpin',
    description:
      'Read the terms governing access to and use of the Helpin website, workspace, AI agents, and connected services.',
    canonicalPath: '/terms',
    imagePath: '/og/helpin-terms-green-v5.png',
    imageAlt: 'Helpin Terms of Service',
  },
} as const satisfies Record<string, PageSeo>;

export function createPageMetadata(page: PageSeo): Metadata {
  return {
    title: page.title,
    description: page.description,
    alternates: { canonical: page.canonicalPath },
    openGraph: {
      title: page.title,
      description: page.description,
      url: page.canonicalPath,
      siteName: 'Helpin',
      locale: 'en_US',
      type: 'website',
      images: [
        {
          url: page.imagePath,
          width: 1200,
          height: 630,
          type: 'image/png',
          alt: page.imageAlt,
        },
      ],
    },
    twitter: {
      card: 'summary_large_image',
      title: page.title,
      description: page.description,
      images: [{ url: page.imagePath, alt: page.imageAlt }],
    },
    robots: {
      index: true,
      follow: true,
      googleBot: {
        index: true,
        follow: true,
        'max-image-preview': 'large',
        'max-snippet': -1,
        'max-video-preview': -1,
      },
    },
  };
}
