import Link from 'next/link';
import { ArrowRight, ChevronRight } from 'lucide-react';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { SectionHead, SIGNUP_URL } from '../_components/ui';
import { COMPETITORS } from './compare-data';
import './compare.css';

export const metadata = createPageMetadata(PAGE_SEO.compare);

export default function CompareHub() {
  return (
    <>
      <PreviewNav />
      <div className="compare-page">
        <section className="compare-hero">
          <HeroVortex variant="orbit" tone="dark" />
          <div className="wrap">
            <div className="compare-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Compare</span></div>
            <span className="eyebrow">Compare Helpin</span>
            <h1>How Helpin compares with the tools you know.</h1>
            <p className="lede">Honest, side-by-side comparisons with the support desks and project tools teams use today, including where the other tool is the better fit.</p>
          </div>
        </section>
        <section id="comparisons">
          <div className="wrap">
            <SectionHead eyebrow="Comparisons" title="Pick the tool you’re weighing up." lede="Each page covers features, pricing for a sample team, what switching involves, and the sources we checked." />
            <div className="compare-cards">
              {COMPETITORS.map(item => (
                <Link key={item.slug} className="compare-card" href={`/compare/${item.slug}`}>
                  <span>{item.category}</span>
                  <strong>Helpin vs {item.name}</strong>
                  <ArrowRight size={16} aria-hidden="true" />
                  <p>{item.cardLine}</p>
                </Link>
              ))}
            </div>
          </div>
        </section>
        <PlatformClosing id="compare-hub-final-title" eyebrow="Try it yourself" title="See the difference in your own workspace." description="Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free." primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
