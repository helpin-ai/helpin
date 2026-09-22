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
    title: 'Helpin — Support, projects, CRM and AI agents',
    description:
      'Helpin connects projects, support, sales, and docs with AI agents that plan, build, triage, and follow up across every team.',
    canonicalPath: '/',
    imagePath: '/og/helpin-home-green-v3.png',
    imageAlt: 'Helpin — Bring every team together. Put AI agents to work.',
  },
  pricing: {
    title: 'Pricing — Open source and Helpin Cloud',
    description:
      'Self-host the open-source product or choose Helpin Cloud with unlimited teammates and monthly AI usage. Compare plans, hosting and Enterprise licensing.',
    canonicalPath: '/pricing',
    imagePath: '/og/helpin-pricing-green-v3.png',
    imageAlt: 'Helpin pricing — Every module. Every teammate. One price.',
  },
  privacy: {
    title: 'Privacy Policy — Helpin',
    description:
      'Learn how Helpin collects, protects, and processes information across the website and connected workspace.',
    canonicalPath: '/privacy',
    imagePath: '/og/helpin-privacy-green-v3.png',
    imageAlt: 'Privacy at Helpin',
  },
  terms: {
    title: 'Terms of Service — Helpin',
    description:
      'Read the terms governing access to and use of the Helpin website, workspace, AI agents, and connected services.',
    canonicalPath: '/terms',
    imagePath: '/og/helpin-terms-green-v3.png',
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
          secureUrl: page.imagePath,
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
