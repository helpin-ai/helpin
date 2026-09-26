import Link from 'next/link';
import {
  ArrowRight, ArrowUpRight, Award, BarChart3, Blocks, BookOpenCheck, Bot, CalendarCheck, Check, ChevronRight, Cloud, Cpu, Feather,
  GitBranch, GitPullRequest, Handshake, Import, Inbox, type LucideIcon, Megaphone, MessagesSquare, Minus, PenLine, PlayCircle, RefreshCw,
  Rocket, Scale, Server, ShieldCheck, Smartphone, Sprout, Users, Wallet, X, Zap,
} from 'lucide-react';
import { SITE_URL } from '@/lib/metadata';
import { article, JsonLd, organization } from '@/lib/structured-data';
import { CustomerLogos } from '../_components/CustomerLogos';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaNote, CtaRow, DEMO_URL, FAQList, GITHUB_URL, SIGNUP_URL } from '../_components/ui';
import { ARTICLES, HELPIN_ARTICLE } from './article-data';
import { cellStatus, COMPETITORS, competitorSeo, compareVideo, formatChecked, type Cell, type Competitor, type IconKey, type Status } from './compare-data';
import { CompareFilm } from './CompareFilm';
import { HeroMatchup } from './HeroMatchup';
import { InViewOnce } from './InViewOnce';
import { LazyPreview } from './LazyPreview';
import { PriceCalculator } from './PriceCalculator';
import { Toc } from './Toc';

const ICONS: Record<IconKey, LucideIcon> = {
  workflow: GitPullRequest, billing: Wallet, team: Users, hosting: Server, crm: Handshake, open: GitBranch,
  maturity: Sprout, requests: Inbox, agents: Bot, loop: RefreshCw, channels: MessagesSquare, ecosystem: Blocks,
  enterprise: ShieldCheck, reporting: BarChart3, messaging: Megaphone, simplicity: Feather, import: Import,
  mobile: Smartphone, community: Award, deploy: Rocket, speed: Zap, models: Cpu, docs: BookOpenCheck,
};

const STATUS: Record<Status, { Icon: LucideIcon; label: string }> = {
  yes: { Icon: Check, label: 'Yes' },
  partial: { Icon: Minus, label: 'Partly' },
  no: { Icon: X, label: 'No' },
};

export function HelpinMark({ size = 20, tone = 'light' }: { size?: number; tone?: 'light' | 'dark' }) {
  return <img className="cmp-mark" src={tone === 'dark' ? '/brand/helpin-icon-white.svg' : '/brand/helpin-icon-ink.svg'} width={size} height={size} alt="" />;
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

/** One product's side of a comparison, labeled so skimmers can follow either column. */
function Side({ name, children }: { name: string; children: React.ReactNode }) {
  const helpin = name === 'Helpin';
  return (
    <div className="cmp-side" data-product={helpin ? 'helpin' : 'rival'}>
      <span className="cmp-side-name">{helpin ? <HelpinMark size={18} /> : null}{name}</span>
      <p>{children}</p>
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

// Both products' take on one topic in a single frame, with the verdict as its footer.
function Versus({ name, competitor, helpin, verdict }: { name: string; competitor: React.ReactNode; helpin: React.ReactNode; verdict?: string }) {
  return (
    <div className="cmp-versus">
      <div className="cmp-sides">
        <Side name={name}>{competitor}</Side>
        <span className="cmp-versus-badge" aria-hidden="true">vs</span>
        <Side name="Helpin">{helpin}</Side>
      </div>
      {verdict ? <p className="cmp-verdict"><Scale size={15} aria-hidden="true" /><span><b>The difference:</b> {verdict}</span></p> : null}
    </div>
  );
}

// The same product two ways: hosted by Helpin, or the open-source Community edition.
function Editions() {
  return (
    <div className="cmp-editions">
      <article>
        <header><Cloud size={20} aria-hidden="true" /><div><h3>Helpin Cloud</h3><span>Hosted by Helpin</span></div></header>
        <ul>
          {['One price per workspace, unlimited teammates', 'AI usage allowance included in every plan', 'Your own AI keys on Enterprise', 'Managed by Helpin, with support'].map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}
        </ul>
        <a className="btn btn-primary" href={SIGNUP_URL}>Start free trial<ArrowRight size={15} aria-hidden="true" /></a>
        <small>14 days, no card required</small>
      </article>
      <article>
        <header><GitBranch size={20} aria-hidden="true" /><div><h3>Community edition</h3><span>Open source · AGPL-3.0 · 0.2 beta</span></div></header>
        <ul>
          {['Free, with no plan limits or seat counts', 'Every product, coding agents included', 'Runs on your own AI provider and keys', 'Docker Compose on your own servers'].map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}
        </ul>
        <Link className="btn btn-secondary" href="/self-hosting">Self-host free<ArrowRight size={15} aria-hidden="true" /></Link>
        <small><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">View the code on GitHub</a></small>
      </article>
    </div>
  );
}

function ComparisonTable({ competitor }: { competitor: Competitor }) {
  const { name } = competitor;
  return (
    <div className="included-table-scroll cmp-table-scroll" role="region" aria-label={`Helpin and ${name} feature comparison`} tabIndex={0}>
      <table className="included-table cmp-table" role="table">
        <caption className="sr-only">Helpin and {name} compared. {competitor.tableLede}</caption>
        <colgroup><col className="cmp-table-feature" /><col className="cmp-table-helpin" /><col /></colgroup>
        <thead role="rowgroup">
          <tr role="row">
            <th scope="col" role="columnheader">Capability</th>
            <th scope="col" role="columnheader" className="cmp-table-product"><span className="cmp-table-name"><HelpinMark size={18} />Helpin</span><span>Open source · Cloud or self-hosted</span></th>
            <th scope="col" role="columnheader" className="cmp-table-product"><span className="cmp-table-name">{name}</span><span>{competitor.category}</span></th>
          </tr>
        </thead>
        {competitor.table.map(group => (
          <tbody key={group.group} role="rowgroup">
            <tr role="row" className="cmp-table-group"><th scope="rowgroup" role="rowheader" colSpan={3}>{group.group}</th></tr>
            {group.rows.map(row => (
              <tr key={row.label} role="row">
                <th scope="row" role="rowheader">{row.label}</th>
                <td role="cell" className="cmp-table-helpin-cell"><Value value={row.helpin} status={row.helpinStatus} /></td>
                <td role="cell"><Value value={row.competitor} status={row.competitorStatus} /></td>
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

export function ComparePage({ competitor }: { competitor: Competitor }) {
  const { name } = competitor;
  const copy = ARTICLES[competitor.slug];
  const others = COMPETITORS.filter(item => item.slug !== competitor.slug);
  const checked = formatChecked(competitor.checked);
  const seo = competitorSeo(competitor);
  const support = competitor.group === 'Customer support';
  const video = compareVideo(competitor);
  const toc = [
    { id: 'overview', label: 'At a glance' },
    { id: 'why-switch', label: 'Why teams switch' },
    { id: 'features', label: 'Core features' },
    { id: 'pricing', label: 'Pricing and value' },
    { id: 'hosting', label: 'Cloud or open source' },
    { id: 'switching', label: 'Switching' },
    { id: 'strengths', label: `Where ${name} is stronger` },
    { id: 'fit', label: 'Which one fits' },
    { id: 'questions', label: 'Questions' },
  ];
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
      {
        '@type': 'VideoObject',
        name: video.title,
        description: video.summary,
        thumbnailUrl: `${SITE_URL}${video.poster}`,
        contentUrl: `${SITE_URL}${video.src}`,
        uploadDate: video.published,
        duration: `PT${video.seconds}S`,
      },
      organization,
    ],
  };

  return (
    <>
      <PreviewNav tone="dark" />
      <div className="platform-page compare-page">
        <JsonLd data={structured} />
        <section className="platform-hero motion-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><Link href="/compare">Compare</Link><ChevronRight size={12} aria-hidden="true" /><span>{name}</span></div>
            <div className="platform-hero-grid">
              <div className="platform-hero-copy">
                <span className="eyebrow">Open-source {name} alternative · {competitor.category}</span>
                <h1>Helpin vs {name}: <span>{competitor.hero.accent}</span></h1>
                <p className="lede">{competitor.hero.lede}</p>
                <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Talk to us about switching" />
                <CtaNote trial support />
                <div className="platform-hero-points">
                  <a href="#video"><PlayCircle size={14} aria-hidden="true" />Watch in {video.seconds} seconds</a>
                  <span><GitBranch size={14} aria-hidden="true" />Open source · Cloud or self-hosted</span>
                  <span><PenLine size={14} aria-hidden="true" />By the Helpin team</span>
                  <span><CalendarCheck size={14} aria-hidden="true" />Verified {checked}</span>
                </div>
              </div>
              <HeroMatchup name={name} glance={competitor.glance} />
            </div>
            <CompareFilm id="video" name={name} video={video} />
          </div>
        </section>

        <ProofStrip />

        <section className="cmp-article-section">
          <div className="wrap cmp-article">
            <aside className="cmp-article-aside">
              <Toc items={toc} />
              <div className="cmp-aside-cta">
                <strong>Try Helpin free</strong>
                <p>14-day Cloud trial, no card. Or self-host the open-source Community edition for free.</p>
                <a className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</a>
                <Link className="cmp-aside-link" href="/self-hosting">Self-host free<ArrowRight size={13} aria-hidden="true" /></Link>
              </div>
            </aside>

            <article className="cmp-article-body">
              <section id="overview" className="cmp-block">
                <h2>Helpin vs {name} at a glance</h2>
                {copy.intro.map(paragraph => <p key={paragraph}>{paragraph}</p>)}
                <div className="cmp-callout">
                  <strong>The main difference is what each product is built around:</strong>
                  <ul>{copy.difference.map(item => <li key={item.lead}><b>{item.lead}</b> {item.text}</li>)}</ul>
                </div>
                <p>Here’s how the two compare across the capabilities teams ask about most. {competitor.tableLede}</p>
                <ComparisonTable competitor={competitor} />
                <p className="cmp-small">Comparison data verified {checked}.</p>
              </section>

              <section id="why-switch" className="cmp-block">
                <h2>{support ? `Why support teams switch from ${name}` : `Why teams look beyond ${name}`}</h2>
                <p>{support ? 'Helpin is open source, on Helpin Cloud or your own servers. Beyond that, these are the four differences support teams notice first.' : 'Four common reasons teams give for looking elsewhere, and how Helpin handles each one.'}</p>
                <div className="cmp-diff-grid">
                  {competitor.reasons.map((reason, index) => {
                    const Icon = ICONS[reason.icon];
                    return (
                      <InViewOnce as="article" key={reason.title} className="cmp-diff-card">
                        <div className="cmp-diff-art" role="img" aria-label={`${reason.competitorLane ? `${name}: ${reason.competitorLane.join(', then ')}. ` : ''}Helpin: ${reason.helpinLane.join(', then ')}.`}>
                          {reason.competitorLane ? <Lane label={name} steps={reason.competitorLane} /> : null}
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
              </section>

              <section id="features" className="cmp-block">
                <h2>Core feature comparison</h2>
                <p>Feature names can sound alike across tools. What matters is how each product handles the work, and what your team does next.</p>
                {copy.features.map(feature => (
                  <div key={feature.id} id={feature.id} className="cmp-feature">
                    <h3>{feature.title}</h3>
                    <Versus name={name} competitor={feature.competitor} helpin={feature.helpin} verdict={feature.verdict} />
                    {feature.preview ? <LazyPreview product={feature.preview.product} label={feature.preview.label} /> : null}
                  </div>
                ))}
              </section>


              <section id="pricing" className="cmp-block">
                <h2>Pricing and value</h2>
                <Versus name={name} competitor={copy.pricing} helpin={HELPIN_ARTICLE.pricing} />
                <h3 className="cmp-subhead">Estimate what your team would pay</h3>
                <p>Change the team size, plans and AI volume to match your team. The estimate uses list prices and says plainly when {name} costs less. <Link href="/pricing">See Helpin pricing</Link>.</p>
                <PriceCalculator name={name} calculator={competitor.calculator} />
              </section>

              <section id="hosting" className="cmp-block">
                <h2>Cloud or open source</h2>
                <p>Helpin is the same product either way. Let us run it on Helpin Cloud, or self-host the open-source Community edition on your own servers.</p>
                <Versus name={name} competitor={copy.hosting} helpin={HELPIN_ARTICLE.hosting} />
                <Editions />
              </section>

              <section id="switching" className="cmp-block">
                <h2>Switching and onboarding</h2>
                <Versus name={name} competitor={copy.onboarding} helpin={HELPIN_ARTICLE.onboarding} />
                <h3 className="cmp-subhead">{competitor.switching.title}</h3>
                <p>{competitor.switching.lede}</p>
                <div className="cmp-checklists">
                  <article>
                    <h4>Take from {name}</h4>
                    <ul>{competitor.switching.take.map(item => <li key={item}><ArrowRight size={14} aria-hidden="true" />{item}</li>)}</ul>
                  </article>
                  <article>
                    <h4><HelpinMark size={22} />Set up in Helpin</h4>
                    <ul>{competitor.switching.setUp.map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}</ul>
                  </article>
                </div>
                <InViewOnce as="ol" className="cmp-steps">
                  {competitor.switching.steps.map((step, index) => (
                    <li key={step.title} style={{ '--i': index } as React.CSSProperties}>
                      <span className="cmp-step-number">{String(index + 1).padStart(2, '0')}</span>
                      <div>
                        <div className="cmp-step-head"><h4>{step.title}</h4><span className="cmp-status" data-status={step.status}>{step.status}</span></div>
                        <p>{step.body}</p>
                      </div>
                    </li>
                  ))}
                </InViewOnce>
                <a className="btn btn-secondary cmp-switch-cta" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to us about switching<ArrowUpRight size={15} aria-hidden="true" /></a>
              </section>

              <section id="strengths" className="cmp-block">
                <h2>Where {name} is stronger</h2>
                <p>{name} is a good product. These are the areas where it’s ahead of Helpin today.</p>
                <div className="cmp-strength-list">
                  {competitor.strengths.map(strength => {
                    const Icon = ICONS[strength.icon];
                    return <article key={strength.title}><Icon size={22} aria-hidden="true" /><div><h3>{strength.title}</h3><p>{strength.body}</p></div></article>;
                  })}
                </div>
              </section>

              <section id="fit" className="cmp-block">
                <h2>Which one fits your team?</h2>
                <p>{competitor.summary.lede}</p>
                <div className="cmp-choose">
                  <article className="cmp-choose-rival">
                    <header><h3>Choose {name} if…</h3></header>
                    <ul>{competitor.summary.competitor.map(item => <li key={item}><ArrowRight size={15} aria-hidden="true" />{item}</li>)}</ul>
                  </article>
                  <article className="cmp-choose-helpin">
                    <header><HelpinMark size={30} tone="dark" /><h3>Choose Helpin if…</h3></header>
                    <ul>{competitor.summary.helpin.map(item => <li key={item}><Check size={15} strokeWidth={2.2} aria-hidden="true" />{item}</li>)}</ul>
                  </article>
                </div>
              </section>

              <section id="questions" className="cmp-block">
                <h2>Questions about Helpin vs {name}</h2>
                <FAQList items={competitor.faqs} className="platform-faqs" />
              </section>

              <p className="cmp-byline">Written by the Helpin team from {name}’s public pricing and documentation, verified {checked}. Prices are in US dollars and exclude tax. Products change, so check {name}’s site for current details. {name} is a trademark of its owner; Helpin is not affiliated with it. Spot something out of date? Email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a>.</p>
            </article>
          </div>
        </section>

        <section id="more-comparisons" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro">
              <div className="sec-head"><span className="eyebrow">More comparisons</span><h2>See how Helpin compares with other tools.</h2></div>
              <Link className="platform-text-link" href="/compare">All comparisons<ArrowRight size={15} aria-hidden="true" /></Link>
            </div>
            <div className="cmp-card-grid">{others.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
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
      <span className="cmp-card-category">{competitor.category}</span>
      <strong>Helpin vs {competitor.name}</strong>
      <p>{competitor.cardLine}</p>
      <span className="cmp-card-link">Read the comparison<ArrowRight size={14} aria-hidden="true" /></span>
    </Link>
  );
}
