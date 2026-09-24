import type { Metadata } from 'next';
import { createPageMetadata } from '../../../lib/metadata.ts';

const POSITIONING = 'Support, projects, CRM and docs on one customer history, with AI agents.';
export function marketingMetadata(title: string, path: string, description = POSITIONING): Metadata {
  const slug = path.split('/').filter(Boolean).at(-1);
  return createPageMetadata({
    title,
    description,
    canonicalPath: path,
    imagePath: `/og/helpin-${slug}-green-v4.png`,
    imageAlt: title,
  });
}
