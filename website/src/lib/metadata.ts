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
    title: 'Helpin — Open-source AI customer support, CRM and projects',
    description:
      'Helpin is an open-source workspace where AI agents answer customers, plan work and update the CRM using the full customer history. Self-host free or use Cloud.',
    canonicalPath: '/',
    imagePath: '/og/helpin-new-home-green-v4.png',
    imageAlt: 'Helpin — AI agents that do more than answer.',
  },
  product: {
    title: 'Product overview: support, projects, CRM and docs — Helpin',
    description:
      'See how Helpin connects support, meetings, projects, CRM, docs and AI agents on one customer history, so every answer and task comes with the full context.',
    canonicalPath: '/product',
    imagePath: '/og/helpin-product-green-v4.png',
    imageAlt: 'Helpin product overview',
  },
  customerSupport: {
    title: 'AI customer support software with a shared inbox — Helpin',
    description:
      'Chat, email and customer history in one inbox. AI agents answer from your docs and hand off to your team with everything they found. No per-seat fees.',
    canonicalPath: '/products/customer-support',
    imagePath: '/og/helpin-customer-support-green-v4.png',
    imageAlt: 'Helpin customer support',
  },
  meetings: {
    title: 'AI meeting notetaker that turns calls into tasks — Helpin',
    description:
      'Record Google Meet, Zoom, Teams and Webex calls. Get transcripts, summaries, decisions and action items, then turn them into tasks linked to the deal.',
    canonicalPath: '/products/meetings',
    imagePath: '/og/helpin-meetings-green-v4.png',
    imageAlt: 'Helpin meetings',
  },
  projects: {
    title: 'Project management with AI coding agents — Helpin',
    description:
      'Roadmaps, sprints and objectives in one workspace. AI agents plan and code from the task and the customer conversation behind it. Your team approves merges.',
    canonicalPath: '/products/projects',
    imagePath: '/og/helpin-projects-green-v4.png',
    imageAlt: 'Helpin projects',
  },
  crm: {
    title: 'AI CRM with the full customer history — Helpin',
    description:
      'Manage contacts, companies and deals with the email, meetings, support conversations and project work behind them. AI prepares you for every call.',
    canonicalPath: '/products/crm',
    imagePath: '/og/helpin-crm-green-v4.png',
    imageAlt: 'Helpin CRM',
  },
  knowledge: {
    title: 'Knowledge base and help center software — Helpin',
    description:
      'Publish help articles, product guides and API docs on your own domain. AI agents draft updates from support gaps and shipped changes for your team to review.',
    canonicalPath: '/products/knowledge',
    imagePath: '/og/helpin-knowledge-green-v4.png',
    imageAlt: 'Helpin knowledge',
  },
  aiAgents: {
    title: 'AI agents for support, projects and CRM — Helpin',
    description:
      'AI agents that answer customers, plan work, open pull requests and update the CRM from the same customer history your team sees, with approvals you control.',
    canonicalPath: '/products/ai-agents',
    imagePath: '/og/helpin-ai-agents-green-v4.png',
    imageAlt: 'Helpin AI agents',
  },
  developers: {
    title: 'Developers: widget SDKs, MCP server and CLI — Helpin',
    description:
      'Embed the Helpin widget with the JavaScript, React, Next.js or Vue SDK, identify customers, and connect AI tools and agents through MCP and the CLI.',
    canonicalPath: '/developers',
    imagePath: '/og/helpin-developers-green-v4.png',
    imageAlt: 'Build on Helpin',
  },
  selfHosting: {
    title: 'Self-hosted open-source customer support platform — Helpin',
    description:
      'Run the complete Helpin product on your own infrastructure with Docker Compose. AGPL-3.0, no license fee, unlimited users, and your choice of AI provider.',
    canonicalPath: '/self-hosting',
    imagePath: '/og/helpin-self-hosting-green-v4.png',
    imageAlt: 'Self-host Helpin',
  },
  branding: {
    title: 'Brand kit: logo, colors and typography — Helpin',
    description:
      'Download the Helpin logo, symbols, color palette and typography for articles, integrations and presentations.',
    canonicalPath: '/branding',
    imagePath: '/og/helpin-branding-green-v4.png',
    imageAlt: 'Helpin brand kit',
  },
  pricing: {
    title: 'Helpin pricing — Free to self-host, Cloud with AI included',
    description:
      'Self-host Helpin free, or choose Helpin Cloud from $79/month billed annually, with AI usage included. One price per workspace, no per-seat fees.',
    canonicalPath: '/pricing',
    imagePath: '/og/helpin-pricing-green-v4.png',
    imageAlt: 'Helpin pricing — Every module. Every teammate. One price.',
  },
  compare: {
    title: 'Compare Helpin with Intercom, Zendesk, Linear and more',
    description:
      'Side-by-side comparisons of Helpin with Intercom, Zendesk, Help Scout, Chatwoot and Linear: features, pricing for a sample team, and what switching involves.',
    canonicalPath: '/compare',
    imagePath: '/og/helpin-compare-green-v4.png',
    imageAlt: 'Compare Helpin',
  },
  privacy: {
    title: 'Privacy Policy — Helpin',
    description:
      'Learn how Helpin collects, protects, and processes information across the website and connected workspace.',
    canonicalPath: '/privacy',
    imagePath: '/og/helpin-privacy-green-v4.png',
    imageAlt: 'Privacy at Helpin',
  },
  terms: {
    title: 'Terms of Service — Helpin',
    description:
      'Read the terms governing access to and use of the Helpin website, workspace, AI agents, and connected services.',
    canonicalPath: '/terms',
    imagePath: '/og/helpin-terms-green-v4.png',
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
