import type { Metadata } from 'next';
import { MarketingShell } from '../(site)/_components/MarketingShell';
import { PreviewNav } from '../(site)/_components/PreviewNav';
import { PreviewFooter } from '../(site)/_components/PreviewFooter';
import './pricing.css';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.pricing);

export default function PricingLayout({ children }: { children: React.ReactNode }) {
  return <MarketingShell><PreviewNav />{children}<PreviewFooter homepage /></MarketingShell>;
}
