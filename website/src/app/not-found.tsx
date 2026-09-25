import type { Metadata } from 'next';
import Link from 'next/link';
import { MarketingShell } from './(site)/_components/MarketingShell';
import { PreviewNav } from './(site)/_components/PreviewNav';
import { PreviewFooter } from './(site)/_components/PreviewFooter';

export const metadata: Metadata = {
  title: 'Page not found — Helpin',
  robots: { index: false, follow: true },
};

export default function NotFound() {
  return (
    <MarketingShell>
      <PreviewNav />
      <section className="not-found">
        <div className="wrap">
          <span className="eyebrow">404</span>
          <h1>We couldn’t find that page.</h1>
          <p className="lede">The link may be old, or the page may have moved.</p>
          <div className="cta-row">
            <Link className="btn btn-primary" href="/">Go to the homepage →</Link>
            <Link className="btn btn-secondary" href="/product">Explore the product →</Link>
          </div>
        </div>
      </section>
      <PreviewFooter />
    </MarketingShell>
  );
}
