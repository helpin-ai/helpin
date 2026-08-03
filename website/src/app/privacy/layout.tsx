import type { Metadata } from 'next';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.privacy);

export default function PrivacyLayout({ children }: { children: React.ReactNode }) {
  return children;
}
