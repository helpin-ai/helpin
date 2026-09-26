import Link from 'next/link';
import {
  ArrowRight, ArrowUpRight, Award, BarChart3, Blocks, Bot, CalendarCheck, Check, ChevronRight, Feather, GitBranch,
  GitPullRequest, Handshake, Import, Inbox, type LucideIcon, Megaphone, MessagesSquare, Minus, PenLine, RefreshCw,
  Rocket, Scale, Server, ShieldCheck, Smartphone, Sprout, Users, Wallet, X, Zap,
} from 'lucide-react';
import { SITE_URL } from '@/lib/metadata';
import { article, JsonLd, organization } from '@/lib/structured-data';
import { CustomerLogos } from '../_components/CustomerLogos';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CommunityShowcase } from '../_components/platform/PlatformProof';
import { CtaNote, CtaRow, DEMO_URL, FAQList, GITHUB_URL, SectionHead, SIGNUP_URL } from '../_components/ui';
import { ALTERNATIVES } from './alternatives-data';
import { cellStatus, COMPETITORS, competitorSeo, formatChecked, type Cell, type Competitor, type IconKey, type Status } from './compare-data';
import { HeroMatchup } from './HeroMatchup';
import { InViewOnce } from './InViewOnce';
import { PriceCalculator } from './PriceCalculator';

const ICONS: Record<IconKey, LucideIcon> = {
  workflow: GitPullRequest, billing: Wallet, team: Users, hosting: Server, crm: Handshake, open: GitBranch,
  maturity: Sprout, requests: Inbox, agents: Bot, loop: RefreshCw, channels: MessagesSquare, ecosystem: Blocks,
  enterprise: ShieldCheck, reporting: BarChart3, messaging: Megaphone, simplicity: Feather, import: Import,
  mobile: Smartphone, community: Award, deploy: Rocket, speed: Zap,
};

const STATUS: Record<Status, { Icon: LucideIcon; label: string }> = {
  yes: { Icon: Check, label: 'Yes' },
  partial: { Icon: Minus, label: 'Partly' },
  no: { Icon: X, label: 'No' },
};

export function HelpinMark({ size = 20, tone = 'light' }: { size?: number; tone?: 'light' | 'dark' }) {
  return <img className="cmp-mark" src={tone === 'dark' ? '/brand/helpin-icon-white.svg' : '/brand/helpin-icon-ink.svg'} width={size} height={size} alt="" />;
}

// A neutral initial, never the other company's logo.
export function Monogram({ name }: { name: string }) {
  return <span className="cmp-monogram" aria-hidden="true">{name.charAt(0)}</span>;
}

function Value({ value, status }: { value: Cell; status?: Status }) {
  const resolved = cellStatus(value, status);
  const mark = resolved ? (() => {
    const { Icon, label } = STATUS[resolved];
    return <span className="cmp-status-icon" data-status={resolved}><Icon size={13} strokeWidth={2.6} aria-hidden="true" />{typeof value === 'string' ? null : <span className="sr-only">{label}</span>}</span>;
  })() : null;
  if (typeof value !== 'string') return mark;
  return mark ? <span className="cmp-cell">{mark}<span>{value}</span></span> : <>{value}</>;
}

function Lane({ label, steps, helpin }: { label: string; steps: string[]; helpin?: boolean }) {
  return (
    <div className={helpin ? 'cmp-lane cmp-lane-helpin' : 'cmp-lane'}>
      <span className="cmp-lane-label">{helpin ? <HelpinMark size={12} tone="dark" /> : null}{label}</span>
      <ol>{steps.map((step, index) => <li key={step} style={{ '--i': index } as React.CSSProperties}>{helpin && index === steps.length - 1 ? <Check size={11} strokeWidth={2.4} aria-hidden="true" /> : null}{step}</li>)}</ol>
    </div>
  );
}

function ProofStrip() {
  return (
    <section className="cmp-proof" aria-label="Who uses Helpin">
      <div className="wrap">
        <CustomerLogos />
        <ul className="cmp-proof-facts">
          <li><GitBranch size={14} aria-hidden="true" />Open source under AGPL-3.0</li>
          <li><Server size={14} aria-hidden="true" />Self-host free, or use Helpin Cloud</li>
          <li><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">View the code on GitHub<ArrowUpRight size={13} aria-hidden="true" /></a></li>
        </ul>
      </div>
    </section>
  );
}

export function ComparePage({ competitor }: { competitor: Competitor }) {
  const { name } = competitor;
  const others = COMPETITORS.filter(item => item.slug !== competitor.slug);
  const checked = formatChecked(competitor.checked);
  const seo = competitorSeo(competitor);
  const structured = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          { '@type': 'ListItem', position: 1, name: 'Helpin', item: SITE_URL },
          { '@type': 'ListItem', position: 2, name: 'Compare', item: `${SITE_URL}/compare` },
          { '@type': 'ListItem', position: 3, name: `Helpin vs ${name}`, item: `${SITE_URL}${seo.canonicalPath}` },
        ],
      },
      { ...article({ headline: seo.title, description: seo.description, path: seo.canonicalPath, date: competitor.checked }), image: `${SITE_URL}${seo.imagePath}` },
      organization,
    ],
  };

  return (
    <>
      <PreviewNav />
      <div className="platform-page compare-page">
        <JsonLd data={structured} />
        <section className="platform-hero motion-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><Link href="/compare">Compare</Link><ChevronRight size={12} aria-hidden="true" /><span>{name}</span></div>
            <div className="platform-hero-grid">
              <div className="platform-hero-copy">
                <span className="eyebrow">{name} alternative · {competitor.category}</span>
                <h1>Helpin vs {name}: <span>{competitor.hero.accent}</span></h1>
                <p className="lede">{competitor.hero.lede}</p>
                <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Talk to us about switching" />
                <CtaNote trial support />
                <div className="platform-hero-points">
                  <span><PenLine size={14} aria-hidden="true" />By the Helpin team</span>
                  <span><CalendarCheck size={14} aria-hidden="true" />Verified {checked}</span>
                  <span><Scale size={14} aria-hidden="true" />Strengths on both sides</span>
                </div>
              </div>
              <HeroMatchup name={name} glance={competitor.glance} />
            </div>
          </div>
        </section>

        <ProofStrip />

        <nav className="platform-page-nav" aria-label="On this page">
          <div className="wrap">
            <strong>Helpin vs {name}</strong>
            <a href="#short-answer">Short answer</a>
            <a href="#why-switch">Why teams switch</a>
            <a href="#side-by-side">Side by side</a>
            <a href="#product">Product</a>
            <a href="#pricing">Pricing</a>
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

        <section id="why-switch" className="platform-dark section-motion">
          <HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Why teams switch" title={`Why teams look beyond ${name}.`} lede="Four common reasons, and how Helpin handles each one." /></div>
            <div className="cmp-diff-grid">
              {competitor.reasons.map((reason, index) => {
                const Icon = ICONS[reason.icon];
                return (
                  <InViewOnce as="article" key={reason.title} className="cmp-diff-card">
                    <div className="cmp-diff-art" role="img" aria-label={`${name}: ${reason.competitorLane.join(', then ')}. Helpin: ${reason.helpinLane.join(', then ')}.`}>
                      <Lane label={name} steps={reason.competitorLane} />
                      <Lane label="Helpin" steps={reason.helpinLane} helpin />
                    </div>
                    <div className="cmp-diff-copy">
                      <div className="cmp-diff-meta"><Icon size={15} aria-hidden="true" /><span>{String(index + 1).padStart(2, '0')}</span></div>
                      <h3>{reason.title}</h3>
                      <p>{reason.body}</p>
                    </div>
                  </InViewOnce>
                );
              })}
            </div>
          </div>
        </section>

        <section id="side-by-side" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="Side by side" title={`Helpin and ${name}, feature by feature.`} lede={competitor.tableLede} /></div>
            <div className="included-table-scroll cmp-table-scroll" role="region" aria-label={`Helpin and ${name} feature comparison`} tabIndex={0}>
              <table className="included-table cmp-table">
                <caption className="sr-only">Helpin and {name} compared. {competitor.tableLede}</caption>
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
                        <td className="cmp-table-helpin-cell"><Value value={row.helpin} status={row.helpinStatus} /></td>
                        <td><Value value={row.competitor} status={row.competitorStatus} /></td>
                      </tr>
                    ))}
                  </tbody>
                ))}
              </table>
            </div>
          </div>
        </section>

        <section id="product">
          <div className="wrap">
            <div className="platform-section-intro">
              <SectionHead eyebrow="What you get" title="See the product, not just the table." lede="Live previews of the Helpin workspace. Everything shown here runs on the same customer history." />
              <a className="platform-text-link" href={SIGNUP_URL}>Try it free for 14 days<ArrowRight size={15} aria-hidden="true" /></a>
            </div>
            <CommunityShowcase initial={competitor.showcase} />
          </div>
        </section>

        <section id="strengths" className="platform-soft">
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

        <section id="pricing">
          <div className="wrap">
            <div className="platform-section-intro">
              <SectionHead eyebrow="Pricing" title="Estimate what your team would pay." lede={`List prices for Helpin and ${name}. Change the team size, plans and AI volume to match your team.`} />
              <Link className="platform-text-link" href="/pricing">See Helpin pricing<ArrowRight size={15} aria-hidden="true" /></Link>
            </div>
            <PriceCalculator name={name} calculator={competitor.calculator} />
          </div>
        </section>

        <section id="switching" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro">
              <SectionHead eyebrow="Switching" title={competitor.switching.title} lede={competitor.switching.lede} />
              <a className="btn btn-secondary cmp-switch-cta" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to us about switching<ArrowUpRight size={15} aria-hidden="true" /></a>
            </div>
            <div className="cmp-switch-grid">
              <div className="cmp-checklists">
                <article>
                  <h3><Monogram name={name} />Take from {name}</h3>
                  <ul>{competitor.switching.take.map(item => <li key={item}><ArrowRight size={14} aria-hidden="true" />{item}</li>)}</ul>
                </article>
                <article>
                  <h3><HelpinMark size={22} />Set up in Helpin</h3>
                  <ul>{competitor.switching.setUp.map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}</ul>
                </article>
              </div>
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
              <div className="cmp-intro-links">
                {ALTERNATIVES.some(item => item.competitor === name) ? <Link className="platform-text-link" href={`/compare/${ALTERNATIVES.find(item => item.competitor === name)!.slug}`}>Best {name} alternatives<ArrowRight size={15} aria-hidden="true" /></Link> : null}
                <Link className="platform-text-link" href="/compare">All comparisons<ArrowRight size={15} aria-hidden="true" /></Link>
              </div>
            </div>
            <div className="cmp-card-grid">{others.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
          </div>
        </section>

        <section id="about-this-comparison" className="cmp-sources">
          <div className="wrap">
            <h2>About this comparison</h2>
            <p>Written by the Helpin team from {name}’s public pricing and documentation, verified {checked}. Prices are in US dollars and exclude tax. Products change, so check {name}’s site for current details. {name} is a trademark of its owner; Helpin is not affiliated with it. Spot something out of date? Email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a>.</p>
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
