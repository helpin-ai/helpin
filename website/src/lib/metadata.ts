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
    title: 'Helpin — The AI Operating System for Modern Work',
    description:
      'Helpin connects projects, support, sales, and docs with AI agents that plan, build, triage, and follow up across every team.',
    canonicalPath: '/',
    imagePath: '/og/helpin-home.png',
    imageAlt: 'Helpin brings every team together and puts AI agents to work',
  },
  pricing: {
    title: 'Pricing — Helpin',
    description:
      'Simple plans with unlimited teammates, every Helpin module, and AI agents included in one connected workspace.',
    canonicalPath: '/pricing',
    imagePath: '/og/helpin-pricing.png',
    imageAlt: 'Helpin pricing focuses on growth, not seat count',
  },
  privacy: {
    title: 'Privacy Policy — Helpin',
    description:
      'Learn how Helpin collects, protects, and processes information across the website and connected workspace.',
    canonicalPath: '/privacy',
    imagePath: '/og/helpin-privacy.png',
    imageAlt: 'Privacy at Helpin',
  },
  terms: {
    title: 'Terms of Service — Helpin',
    description:
      'Read the terms governing access to and use of the Helpin website, workspace, AI agents, and connected services.',
    canonicalPath: '/terms',
    imagePath: '/og/helpin-terms.png',
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
