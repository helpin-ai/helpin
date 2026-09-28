import { PAGE_SEO, SITE_URL } from './metadata.ts';

const GITHUB_URL = 'https://github.com/helpin-ai/helpin';

type JsonLdData = Record<string, unknown>;

// Escape `<` so page copy can never close the script element early.
export function JsonLd({ data }: { data: JsonLdData }) {
  return <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(data).replace(/</g, '\\u003c') }} />;
}

const ORGANIZATION_ID = `${SITE_URL}/#organization`;

export const organization = {
  '@type': 'Organization',
  '@id': ORGANIZATION_ID,
  name: 'Helpin',
  url: SITE_URL,
  logo: `${SITE_URL}/icon-512.png`,
  email: 'hello@helpin.ai',
  sameAs: [GITHUB_URL],
};

export const website = {
  '@type': 'WebSite',
  '@id': `${SITE_URL}/#website`,
  name: 'Helpin',
  url: SITE_URL,
  publisher: { '@id': ORGANIZATION_ID },
};

type Plan = { name: string; price: number; annual: number };

// Prices come from the pricing page data so the markup can't drift from the visible plans.
export function softwareApplication(plans: readonly Plan[]) {
  return {
    '@type': 'SoftwareApplication',
    '@id': `${SITE_URL}/#software`,
    name: 'Helpin',
    url: SITE_URL,
    description: PAGE_SEO.home.description,
    applicationCategory: 'BusinessApplication',
    operatingSystem: 'Web, Linux (self-hosted with Docker)',
    license: 'https://www.gnu.org/licenses/agpl-3.0.html',
    publisher: { '@id': ORGANIZATION_ID },
    offers: [
      { '@type': 'Offer', name: 'Community (self-hosted)', price: 0, priceCurrency: 'USD', url: `${SITE_URL}/self-hosting` },
      ...plans.flatMap(plan => [
        { '@type': 'Offer', name: `${plan.name} (monthly)`, price: plan.price, priceCurrency: 'USD', url: `${SITE_URL}/pricing` },
        { '@type': 'Offer', name: `${plan.name} (billed annually, per month)`, price: plan.annual, priceCurrency: 'USD', url: `${SITE_URL}/pricing` },
      ]),
    ],
  };
}

export function faqPage(items: readonly (readonly [question: string, answer: string, ...unknown[]])[]) {
  return {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: items.map(([question, answer]) => ({
      '@type': 'Question',
      name: question,
      acceptedAnswer: { '@type': 'Answer', text: answer },
    })),
  };
}

/** A dated article by the Helpin team, so freshness and authorship are machine-readable. */
export function article({ headline, description, path, date }: { headline: string; description: string; path: string; date: string }) {
  return {
    '@type': 'Article',
    headline,
    description,
    url: `${SITE_URL}${path}`,
    mainEntityOfPage: `${SITE_URL}${path}`,
    datePublished: date,
    dateModified: date,
    author: { '@type': 'Organization', name: 'Helpin team', url: SITE_URL },
    publisher: { '@id': ORGANIZATION_ID },
    image: `${SITE_URL}/og/helpin-compare-green-v4.png`,
  };
}
