import type { Metadata } from 'next';

const POSITIONING = 'Support, projects, CRM and docs on one customer history, with AI agents.';
export function previewMetadata(title: string, path: string, description = POSITIONING): Metadata {
  return {
    title, description, alternates: { canonical: path },
    openGraph: { title, description, url: path, siteName: 'Helpin', type: 'website' },
    twitter: { card: 'summary_large_image', title, description },
    robots: { index: false, follow: false },
  };
}
