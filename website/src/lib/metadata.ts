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
    title: 'Helpin — AI agents that do more than answer',
    description:
      'Helpin gives AI agents the full customer context to resolve questions, take action, and follow through across support, projects, CRM, meetings, and docs.',
    canonicalPath: '/',
    imagePath: '/og/helpin-new-home-green-v4.png',
    imageAlt: 'Helpin — AI agents that do more than answer.',
  },
  pricing: {
    title: 'Pricing — Open source and Helpin Cloud',
    description:
      'Self-host the open-source product or choose Helpin Cloud with unlimited teammates and monthly AI usage. Compare plans, hosting and Enterprise licensing.',
    canonicalPath: '/pricing',
    imagePath: '/og/helpin-pricing-green-v4.png',
    imageAlt: 'Helpin pricing — Every module. Every teammate. One price.',
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
