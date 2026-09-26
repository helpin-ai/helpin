import type { Metadata } from 'next';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { LegalShell } from '../(site)/_components/LegalShell';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.privacy);

export default function PrivacyLayout({ children }: { children: React.ReactNode }) {
  return <LegalShell>{children}</LegalShell>;
}
