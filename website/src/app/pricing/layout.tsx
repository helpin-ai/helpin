import type { Metadata } from 'next';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.pricing);

export default function PricingLayout({ children }: { children: React.ReactNode }) {
  return children;
}
