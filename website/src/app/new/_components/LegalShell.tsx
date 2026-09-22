import { MarketingShell } from './MarketingShell';
import { PreviewNav } from './PreviewNav';
import { PreviewFooter } from './PreviewFooter';
import './legal.css';

export function LegalShell({ children }: { children: React.ReactNode }) {
  return (
    <MarketingShell>
      <PreviewNav />
      {children}
      <PreviewFooter />
    </MarketingShell>
  );
}
