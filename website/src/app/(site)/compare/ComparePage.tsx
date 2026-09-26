import Link from 'next/link';
import {
  ArrowRight, Award, BarChart3, Blocks, Bot, CalendarCheck, Check, ChevronRight, Feather, GitBranch, GitPullRequest,
  Handshake, Import, Inbox, Link2, type LucideIcon, Megaphone, MessagesSquare, RefreshCw, Rocket, Scale, Server,
  ShieldCheck, Smartphone, Sprout, Users, Wallet, Zap,
} from 'lucide-react';
import { SITE_URL } from '@/lib/metadata';
import { JsonLd } from '@/lib/structured-data';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaRow, DEMO_URL, FAQList, SectionHead, SIGNUP_URL } from '../_components/ui';
import { COMPETITORS, formatChecked, type Bill, type Cell, type Competitor, type IconKey } from './compare-data';
import { HeroMatchup } from './HeroMatchup';
import { InViewOnce } from './InViewOnce';

const ICONS: Record<IconKey, LucideIcon> = {
  workflow: GitPullRequest, billing: Wallet, team: Users, hosting: Server, crm: Handshake, open: GitBranch,
  maturity: Sprout, requests: Inbox, agents: Bot, loop: RefreshCw, channels: MessagesSquare, ecosystem: Blocks,
  enterprise: ShieldCheck, reporting: BarChart3, messaging: Megaphone, simplicity: Feather, import: Import,
  mobile: Smartphone, community: Award, deploy: Rocket, speed: Zap,
};

export function HelpinMark({ size = 20, tone = 'light' }: { size?: number; tone?: 'light' | 'dark' }) {
  return <img className="cmp-mark" src={tone === 'dark' ? '/brand/helpin-icon-white.svg' : '/brand/helpin-icon-ink.svg'} width={size} height={size} alt="" />;
}

// A neutral initial, never the other company's logo.
export function Monogram({ name }: { name: string }) {
  return <span className="cmp-monogram" aria-hidden="true">{name.charAt(0)}</span>;
}

function Value({ value }: { value: Cell }) {
  if (value === true) return <span className="cmp-yes"><Check size={16} strokeWidth={2.2} aria-hidden="true" /><span className="sr-only">Yes</span></span>;
  if (value === false) return <span className="cmp-no"><span aria-hidden="true">—</span><span className="sr-only">No</span></span>;
  return <>{value}</>;
}

function Lane({ label, steps, helpin }: { label: string; steps: string[]; helpin?: boolean }) {
  return (
    <div className={helpin ? 'cmp-lane cmp-lane-helpin' : 'cmp-lane'}>
      <span className="cmp-lane-label">{helpin ? <HelpinMark size={12} tone="dark" /> : null}{label}</span>
      <ol>{steps.map((step, index) => <li key={step} style={{ '--i': index } as React.CSSProperties}>{helpin && index === steps.length - 1 ? <Check size={11} strokeWidth={2.4} aria-hidden="true" /> : null}{step}</li>)}</ol>
    </div>
  );
}

function BillCard({ name, bill, helpin }: { name: string; bill: Bill; helpin?: boolean }) {
  return (
    <article className={helpin ? 'cmp-bill cmp-bill-helpin' : 'cmp-bill'}>
      <header>
        {helpin ? <HelpinMark size={28} /> : <Monogram name={name} />}
        <div><h3>{name}</h3><span>{bill.plan}</span></div>
      </header>
      <dl>{bill.lines.map(([label, value], index) => <div key={label} style={{ '--i': index } as React.CSSProperties}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
      <div className="cmp-bill-total"><span>Estimated monthly</span><strong>{bill.total}</strong></div>
      <p>{bill.totalNote}</p>
    </article>
  );
}

export function ComparePage({ competitor }: { competitor: Competitor }) {
  const { name } = competitor;
  const others = COMPETITORS.filter(item => item.slug !== competitor.slug);
  const checked = formatChecked(competitor.checked);
  const breadcrumbs = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Helpin', item: SITE_URL },
      { '@type': 'ListItem', position: 2, name: 'Compare', item: `${SITE_URL}/compare` },
      { '@type': 'ListItem', position: 3, name: `Helpin vs ${name}`, item: `${SITE_URL}/compare/${competitor.slug}` },
    ],
  };

  return (
    <>
      <PreviewNav />
      <div className="platform-page compare-page">
        <JsonLd data={breadcrumbs} />
        <section className="platform-hero motion-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><Link href="/compare">Compare</Link><ChevronRight size={12} aria-hidden="true" /><span>{name}</span></div>
            <div className="platform-hero-grid">
              <div className="platform-hero-copy">
                <span className="eyebrow">Compare · {competitor.category}</span>
                <h1>Helpin vs {name}: <span>{competitor.hero.accent}</span></h1>
                <p className="lede">{competitor.hero.lede}</p>
                <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Book a demo" />
                <div className="platform-hero-points">
                  <span><CalendarCheck size={14} aria-hidden="true" />Checked {checked}</span>
                  <span><Link2 size={14} aria-hidden="true" /><a href="#sources">{competitor.sources.length} sources</a></span>
                  <span><Scale size={14} aria-hidden="true" />Strengths on both sides</span>
                </div>
              </div>
              <HeroMatchup name={name} glance={competitor.glance} />
            </div>
          </div>
        </section>

        <nav className="platform-page-nav" aria-label="On this page">
          <div className="wrap">
            <strong>Helpin vs {name}</strong>
            <a href="#short-answer">Short answer</a>
            <a href="#side-by-side">Side by side</a>
            <a href="#differences">Differences</a>
            {competitor.cost ? <a href="#pricing">Pricing</a> : null}
            <a href="#switching">Switching</a>
            <a href="#questions">Questions</a>
          </div>
        </nav>

        <section id="short-answer">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="The short answer" title={competitor.summary.title} lede={competitor.summary.lede} /></div>
            <div className="cmp-choose">
              <article className="cmp-choose-rival">
                <header><Monogram name={name} /><h3>Choose {name} if</h3></header>
                <ul>{competitor.summary.competitor.map(item => <li key={item}><ArrowRight size={15} aria-hidden="true" />{item}</li>)}</ul>
              </article>
              <article className="cmp-choose-helpin">
                <header><HelpinMark size={30} tone="dark" /><h3>Choose Helpin if</h3></header>
                <ul>{competitor.summary.helpin.map(item => <li key={item}><Check size={15} strokeWidth={2.2} aria-hidden="true" />{item}</li>)}</ul>
              </article>
            </div>
          </div>
        </section>

        <section id="side-by-side" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="Side by side" title={`Helpin and ${name}, feature by feature.`} lede={competitor.tableLede} /></div>
            <div className="included-table-scroll cmp-table-scroll" role="region" aria-label={`Helpin and ${name} feature comparison`} tabIndex={0}>
              <table className="included-table cmp-table">
                <caption className="sr-only">Helpin and {name} compared. Checkmarks mean available and dashes mean not available.</caption>
                <colgroup><col className="cmp-table-feature" /><col className="cmp-table-helpin" /><col /></colgroup>
                <thead>
                  <tr>
                    <th scope="col">Capability</th>
                    <th scope="col" className="cmp-table-product"><span className="cmp-table-name"><HelpinMark size={18} />Helpin</span><span>Cloud or self-hosted</span></th>
                    <th scope="col" className="cmp-table-product"><span className="cmp-table-name"><Monogram name={name} />{name}</span><span>{competitor.category}</span></th>
                  </tr>
                </thead>
                {competitor.table.map(group => (
                  <tbody key={group.group}>
                    <tr className="cmp-table-group"><th scope="rowgroup" colSpan={3}>{group.group}</th></tr>
                    {group.rows.map(row => (
                      <tr key={row.label}>
                        <th scope="row">{row.label}</th>
                        <td className="cmp-table-helpin-cell"><Value value={row.helpin} /></td>
                        <td><Value value={row.competitor} /></td>
                      </tr>
                    ))}
                  </tbody>
                ))}
              </table>
            </div>
          </div>
        </section>

        <section id="differences" className="platform-dark section-motion">
          <HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Where they differ" title={competitor.differencesTitle} lede={`Four places where Helpin and ${name} make different choices, and why each one matters.`} /></div>
            <div className="cmp-diff-grid">
              {competitor.differences.map((difference, index) => {
                const Icon = ICONS[difference.icon];
                return (
                  <InViewOnce as="article" key={difference.title} className="cmp-diff-card">
                    <div className="cmp-diff-art" role="img" aria-label={`${name}: ${difference.competitorLane.join(', then ')}. Helpin: ${difference.helpinLane.join(', then ')}.`}>
                      <Lane label={name} steps={difference.competitorLane} />
                      <Lane label="Helpin" steps={difference.helpinLane} helpin />
                    </div>
                    <div className="cmp-diff-copy">
                      <div className="cmp-diff-meta"><Icon size={15} aria-hidden="true" /><span>{String(index + 1).padStart(2, '0')}</span></div>
                      <h3>{difference.title}</h3>
                      <p>{difference.body}</p>
                    </div>
                  </InViewOnce>
                );
              })}
            </div>
          </div>
        </section>

        <section id="strengths">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="Fair is fair" title={`Where ${name} is stronger.`} lede={`${name} is a good product. These are the areas where it is ahead of Helpin today.`} /></div>
            <div className="platform-feature-grid cmp-strength-grid">
              {competitor.strengths.map(strength => {
                const Icon = ICONS[strength.icon];
                return <article key={strength.title}><Icon size={24} aria-hidden="true" /><h3>{strength.title}</h3><p>{strength.body}</p></article>;
              })}
            </div>
          </div>
        </section>

        {competitor.cost ? (
          <section id="pricing" className="platform-soft">
            <div className="wrap">
              <div className="platform-section-intro">
                <SectionHead eyebrow="Pricing" title={competitor.cost.title} lede={competitor.cost.lede} />
                <Link className="platform-text-link" href="/pricing">See Helpin pricing<ArrowRight size={15} aria-hidden="true" /></Link>
              </div>
              <InViewOnce className="cmp-bills">
                <BillCard name="Helpin" bill={competitor.cost.helpin} helpin />
                <BillCard name={name} bill={competitor.cost.competitor} />
              </InViewOnce>
              <p className="included-note">{competitor.cost.note}</p>
            </div>
          </section>
        ) : null}

        <section id="switching">
          <div className="wrap platform-split cmp-switch">
            <SectionHead eyebrow="Switching" title={competitor.switching.title} lede={competitor.switching.lede} />
            <InViewOnce as="ol" className="cmp-steps">
              {competitor.switching.steps.map((step, index) => (
                <li key={step.title} style={{ '--i': index } as React.CSSProperties}>
                  <span className="cmp-step-number">{String(index + 1).padStart(2, '0')}</span>
                  <div>
                    <div className="cmp-step-head"><h3>{step.title}</h3><span className="cmp-status" data-status={step.status}>{step.status}</span></div>
                    <p>{step.body}</p>
                  </div>
                </li>
              ))}
            </InViewOnce>
          </div>
        </section>

        <section id="questions">
          <div className="wrap platform-faq-grid">
            <SectionHead eyebrow="Questions" title={`Helpin vs ${name}: common questions.`} />
            <div><FAQList items={competitor.faqs} className="platform-faqs" /></div>
          </div>
        </section>

        <section id="more-comparisons" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro">
              <SectionHead eyebrow="More comparisons" title="See how Helpin compares with other tools." />
              <Link className="platform-text-link" href="/compare">All comparisons<ArrowRight size={15} aria-hidden="true" /></Link>
            </div>
            <div className="cmp-card-grid">{others.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
          </div>
        </section>

        <section id="sources" className="cmp-sources">
          <div className="wrap">
            <h2>Sources</h2>
            <p>Checked {checked}. Prices are in US dollars and exclude tax. Products change, so check {name}’s site for current details. {name} is a trademark of its owner; Helpin is not affiliated with it. Spot something out of date? Email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a>.</p>
            <ul>{competitor.sources.map(source => <li key={source.url}><a href={source.url} target="_blank" rel="noopener noreferrer nofollow">{source.label}</a></li>)}</ul>
          </div>
        </section>

        <PlatformClosing id="compare-final-title" eyebrow="14-day free trial" title={competitor.closing.title} description={competitor.closing.description} primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}

export function CompareCard({ competitor }: { competitor: Competitor }) {
  return (
    <Link className="cmp-card" href={`/compare/${competitor.slug}`}>
      <div className="cmp-card-names"><HelpinMark size={26} /><span>vs</span><Monogram name={competitor.name} /></div>
      <span className="cmp-card-category">{competitor.category}</span>
      <strong>Helpin vs {competitor.name}</strong>
      <p>{competitor.cardLine}</p>
      <span className="cmp-card-link">Read the comparison<ArrowRight size={14} aria-hidden="true" /></span>
    </Link>
  );
}
