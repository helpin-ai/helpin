import type { Metadata } from 'next';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.terms);

export default function TermsLayout({ children }: { children: React.ReactNode }) {
  return children;
}
