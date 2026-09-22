import type { Metadata } from 'next';

const POSITIONING = 'Support, projects, CRM and docs on one customer history, with AI agents.';
export function previewMetadata(title: string, path: string, description = POSITIONING): Metadata {
  const slug = path === '/new' ? 'new-home' : path.split('/').filter(Boolean).at(-1);
  const image = { url: `/og/helpin-${slug}-green-v3.png`, width: 1200, height: 630, type: 'image/png', alt: title };
  return {
    title, description, alternates: { canonical: path },
    openGraph: { title, description, url: path, siteName: 'Helpin', type: 'website', images: [image] },
    twitter: { card: 'summary_large_image', title, description, images: [{ url: image.url, alt: title }] },
    robots: { index: false, follow: false },
  };
}
