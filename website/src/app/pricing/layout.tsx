import type { Metadata } from 'next';
import { MarketingShell } from '../new/_components/MarketingShell';
import { PreviewNav } from '../new/_components/PreviewNav';
import { PreviewFooter } from '../new/_components/PreviewFooter';
import './pricing.css';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata(PAGE_SEO.pricing);

export default function PricingLayout({ children }: { children: React.ReactNode }) {
  return <MarketingShell><PreviewNav />{children}<PreviewFooter homepage /></MarketingShell>;
}
