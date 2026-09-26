import Link from 'next/link';
import { ArrowRight, CalendarCheck, ChevronRight, FileSearch, Link2, RefreshCw, Scale } from 'lucide-react';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaRow, DEMO_URL, SectionHead, SIGNUP_URL } from '../_components/ui';
import { CompareCard, HelpinMark, Monogram } from './ComparePage';
import { COMPETITORS, formatChecked } from './compare-data';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './compare.css';

export const metadata = createPageMetadata(PAGE_SEO.compare);

const GROUPS = ['Customer support', 'Project management'] as const;
const CHECKED = formatChecked(COMPETITORS[0].checked);

export default function CompareHub() {
  return (
    <>
      <PreviewNav />
      <div className="platform-page compare-page">
        <section className="platform-hero motion-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Compare</span></div>
            <div className="platform-hero-grid">
              <div className="platform-hero-copy">
                <span className="eyebrow">Compare Helpin</span>
                <h1>How Helpin compares <span>with the tools you know.</span></h1>
                <p className="lede">Side-by-side comparisons with the support desks and project tools teams use today: features, pricing for a sample team, what switching involves, and where the other tool is the better fit.</p>
                <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Book a demo" />
                <div className="platform-hero-points">
                  <span><CalendarCheck size={14} aria-hidden="true" />Checked {CHECKED}</span>
                  <span><Link2 size={14} aria-hidden="true" />Sources on every page</span>
                  <span><Scale size={14} aria-hidden="true" />Strengths on both sides</span>
                </div>
              </div>
              <nav className="cmp-hub-board" aria-label="Comparisons">
                <div className="cmp-matchup-bar"><span><Scale size={13} aria-hidden="true" />Comparisons</span><span>{COMPETITORS.length} tools</span></div>
                <ul>
                  {COMPETITORS.map(item => (
                    <li key={item.slug}>
                      <Link href={`/compare/${item.slug}`}>
                        <span className="cmp-hub-pair"><HelpinMark size={22} tone="dark" /><Monogram name={item.name} /></span>
                        <span className="cmp-hub-name"><strong>Helpin vs {item.name}</strong><span>{item.category}</span></span>
                        <ArrowRight size={15} aria-hidden="true" />
                      </Link>
                    </li>
                  ))}
                </ul>
              </nav>
            </div>
          </div>
        </section>

        {GROUPS.map((group, index) => {
          const items = COMPETITORS.filter(item => item.group === group);
          const head = <SectionHead
            eyebrow={group}
            title={group === 'Customer support' ? 'Helpin and other support tools.' : 'Helpin and other project tools.'}
            lede={group === 'Customer support'
              ? 'Help desks and shared inboxes, compared on AI, pricing, self-hosting and what happens after the reply.'
              : 'Issue trackers, compared on planning, coding agents, pricing and how customer requests reach the work.'}
          />;
          const cards = <div className="cmp-card-grid">{items.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>;
          return (
            <section key={group} id={group === 'Customer support' ? 'support-tools' : 'project-tools'} className={index ? 'platform-soft' : undefined}>
              {/* A single comparison sits beside its heading rather than alone in a wide grid. */}
              {items.length === 1
                ? <div className="wrap platform-split cmp-group-split">{head}{cards}</div>
                : <div className="wrap"><div className="platform-section-intro">{head}</div>{cards}</div>}
            </section>
          );
        })}

        <section id="method">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="How we compare" title="Written to help you decide." lede="A comparison is only useful if you can trust it, including the parts that don’t favor us." /></div>
            <div className="platform-feature-grid">
              <article><FileSearch size={24} aria-hidden="true" /><h3>Checked against their own pages.</h3><p>Prices and features come from each product’s public pages and docs, with the date we checked and links to every source.</p></article>
              <article><Scale size={24} aria-hidden="true" /><h3>Honest about trade-offs.</h3><p>Every page says where the other tool is stronger today, from channels and mobile SDKs to maturity.</p></article>
              <article><RefreshCw size={24} aria-hidden="true" /><h3>Corrected when things change.</h3><p>Products change quickly. If something is out of date, email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a> and we’ll fix it.</p></article>
            </div>
          </div>
        </section>

        <PlatformClosing id="compare-hub-final-title" eyebrow="14-day free trial" title="See the difference in your own workspace." description="Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free." primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
