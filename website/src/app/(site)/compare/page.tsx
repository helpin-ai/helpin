import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, CalendarCheck, Check, ChevronRight, FileSearch, FolderKanban, Handshake, Inbox, Mic, PenLine, RefreshCw, Scale } from 'lucide-react';
import { createPageMetadata, PAGE_SEO, SITE_URL } from '@/lib/metadata';
import { JsonLd } from '@/lib/structured-data';
import { CustomerLogos } from '../_components/CustomerLogos';
import { HeroVortex } from '../_components/HeroVortex';
import { PreviewNav } from '../_components/PreviewNav';
import { PreviewFooter } from '../_components/PreviewFooter';
import { PlatformClosing } from '../_components/platform/PlatformParts';
import { CtaNote, CtaRow, DEMO_URL, FAQList, SectionHead, SIGNUP_URL } from '../_components/ui';
import { CompareCard, HelpinMark } from './ComparePage';
import { CompareMatrix } from './CompareMatrix';
import { COMPETITORS, formatChecked } from './compare-data';
import { InViewOnce } from './InViewOnce';
import { LazyPreview } from './LazyPreview';
import { MATRIX_TOOLS } from './matrix-data';
import '../_components/platform/platform.css';
import '../_components/platform/platform-polish.css';
import './compare.css';

export const metadata = createPageMetadata(PAGE_SEO.compare);

const CHECKED = formatChecked(COMPETITORS[0].checked);
const MATRIX_COLUMNS = MATRIX_TOOLS.map(slug => COMPETITORS.find(item => item.slug === slug)!).map(({ slug, name, category }) => ({ slug, name, category }));

const REASONS = [
  {
    eyebrow: 'Support and product work',
    title: 'Support that doesn’t stop at the answer.',
    body: 'Chat and email arrive in one inbox with the customer’s history attached. When a question needs a fix, it becomes a task, a coding agent can open a pull request for review, and the team follows up in the same conversation when it ships.',
    link: { href: '/products/customer-support', label: 'Explore Support' },
    preview: { product: 'inbox', label: 'The shared inbox with the earlier conversation, linked task and AI draft in view.' },
  },
  {
    eyebrow: 'Projects',
    title: 'Plan the work with the customer attached.',
    body: 'Roadmaps, sprints and objectives sit next to the requests that shaped them. Agents plan and code from the task and the conversation behind it, and your team approves what merges.',
    link: { href: '/products/projects', label: 'Explore Projects' },
    preview: { product: 'projects', label: 'A task with the customer conversation behind it and the agent’s progress.' },
  },
  {
    eyebrow: 'CRM and meetings',
    title: 'One record for every customer.',
    body: 'Contacts, companies and deals carry the conversations, meeting notes and open work behind them, so the next call starts with the whole story instead of five tabs.',
    link: { href: '/products/crm', label: 'Explore CRM' },
    preview: { product: 'crm', label: 'A deal with the conversations, meetings and work behind it.' },
  },
] as const;

const TOOLS = [
  { Icon: Inbox, name: 'Help desk', examples: 'Like Intercom, Zendesk or Help Scout' },
  { Icon: FolderKanban, name: 'Issue tracker', examples: 'Like Linear' },
  { Icon: Handshake, name: 'CRM', examples: 'Contacts, companies and deals' },
  { Icon: Mic, name: 'Meeting notetaker', examples: 'Calls, decisions and action items' },
  { Icon: BookOpen, name: 'Knowledge base', examples: 'Help center and product docs' },
];

const FAQS = [
  ['Why do teams switch to Helpin?', 'Most switch because their support questions keep turning into product work, deals and follow-ups spread across several tools. Helpin keeps the conversation, the task, the pull request and the customer record on one history, with AI agents that can do the work in between.'],
  ['Which tools can Helpin replace?', 'Helpin covers a shared inbox and help center, project planning, a CRM with deals, meeting notes and AI agents. It can take the place of a separate help desk, issue tracker, CRM and meeting notetaker, or work alongside the tools you keep through MCP.'],
  ['Where is Helpin not the right fit yet?', 'If you need phone, WhatsApp or social channels in the inbox, native mobile SDKs, SSO or SLA policies today, tools such as Intercom, Zendesk and Chatwoot cover more. Each comparison lists where the other tool is stronger.'],
  ['How much does Helpin cost?', 'Helpin charges one price per workspace with unlimited teammates: Starter is $79 and Growth $239 a month billed annually, each with an AI usage allowance included. The self-hosted Community edition is free.'],
  ['Can I bring my data from another tool?', 'Helpin imports Help Scout Docs articles, Shortcut projects and CRM contacts from CSV today. Importers for Zendesk and Intercom are coming soon, and you can run Helpin alongside your current tool while you move.'],
  ['Is Helpin open source?', 'Yes. Every product feature is open source under AGPL-3.0. Run the Community edition (0.1 beta) on your own servers with Docker Compose, or let us host it on Helpin Cloud.'],
] as const;

const HUB_JSON_LD = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: 'Helpin', item: SITE_URL },
        { '@type': 'ListItem', position: 2, name: 'Compare', item: `${SITE_URL}/compare` },
      ],
    },
    {
      '@type': 'CollectionPage',
      name: PAGE_SEO.compare.title,
      description: PAGE_SEO.compare.description,
      url: `${SITE_URL}/compare`,
      mainEntity: {
        '@type': 'ItemList',
        itemListElement: COMPETITORS.map((item, index) => ({ '@type': 'ListItem', position: index + 1, name: `Helpin vs ${item.name}`, url: `${SITE_URL}/compare/${item.slug}` })),
      },
    },
  ],
};

export default function CompareHub() {
  return (
    <>
      <PreviewNav />
      <div className="platform-page compare-page">
        <JsonLd data={HUB_JSON_LD} />
        <section className="platform-hero motion-hero cmp-hub-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">
            <div className="platform-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Compare</span></div>
            <div className="platform-hero-copy">
              <span className="eyebrow">Compare Helpin</span>
              <h1>Why teams switch <span>to Helpin.</span></h1>
              <p className="lede">One customer history for support, projects, CRM, meetings and docs, with AI agents that do the work in between. See how Helpin compares with Intercom, Zendesk, Help Scout, Chatwoot and Linear, including where they’re stronger.</p>
              <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Talk to us about switching" />
              <CtaNote trial support />
              <div className="platform-hero-points">
                <span><PenLine size={14} aria-hidden="true" />By the Helpin team</span>
                <span><CalendarCheck size={14} aria-hidden="true" />Verified {CHECKED}</span>
                <span><Scale size={14} aria-hidden="true" />Strengths on both sides</span>
              </div>
            </div>
          </div>
        </section>

        <section className="cmp-proof" aria-label="Who uses Helpin"><div className="wrap"><CustomerLogos /></div></section>

        <section id="compare-tools">
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Side by side" title="Helpin next to the tools you use today." lede="The capabilities teams ask about most, with Helpin pinned on the left. Rows where Helpin falls short stay in the table." /></div>
            <CompareMatrix tools={MATRIX_COLUMNS} />
            <p className="cmp-mx-note">Verified {CHECKED} from each product’s public pricing and documentation. Open a full comparison for prices, details and trade-offs.</p>
          </div>
        </section>

        <section id="why-teams-switch" className="platform-soft">
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Why teams switch" title="Less switching between tools. More done for the customer." lede="The same history powers every part of Helpin, so the work after the reply doesn’t get lost between products." /></div>
            <div className="cmp-reason-rows">
              {REASONS.map((reason, index) => (
                <article key={reason.title} className={index % 2 ? 'cmp-reason-row cmp-reason-row-flip' : 'cmp-reason-row'}>
                  <div className="cmp-reason-copy">
                    <span className="eyebrow">{reason.eyebrow}</span>
                    <h3>{reason.title}</h3>
                    <p>{reason.body}</p>
                    <Link className="platform-text-link" href={reason.link.href}>{reason.link.label}<ArrowRight size={15} aria-hidden="true" /></Link>
                  </div>
                  <LazyPreview product={reason.preview.product} label={reason.preview.label} />
                </article>
              ))}
            </div>
          </div>
        </section>

        <section id="one-history" className="platform-dark section-motion">
          <HeroVortex variant="converge" tone="dark" />
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="One customer history" title="One workspace instead of five tools." lede="Move the tools you juggle into Helpin, or connect the ones you keep. Either way, the history stays in one place." /></div>
            <InViewOnce className="cmp-merge">
              <ul className="cmp-merge-tools">
                {TOOLS.map(({ Icon, name, examples }, index) => (
                  <li key={name} style={{ '--i': index } as React.CSSProperties}><span><Icon size={17} aria-hidden="true" /></span><div><strong>{name}</strong><small>{examples}</small></div></li>
                ))}
              </ul>
              <svg className="cmp-merge-lines" viewBox="0 0 160 500" preserveAspectRatio="none" aria-hidden="true">
                {[50, 150, 250, 350, 450].map((y, index) => <path key={y} style={{ '--i': index } as React.CSSProperties} d={`M0 ${y} C 80 ${y}, 80 250, 160 250`} pathLength={1} vectorEffect="non-scaling-stroke" />)}
              </svg>
              <div className="cmp-merge-helpin">
                <div className="cmp-merge-head"><HelpinMark size={34} tone="dark" /><div><strong>Helpin</strong><span>One customer history</span></div></div>
                <ul>
                  {['Support inbox and help center', 'Projects, sprints and roadmaps', 'CRM with deals', 'Meeting notes', 'Knowledge and API docs', 'AI agents across all of it'].map(item => <li key={item}><Check size={14} strokeWidth={2.4} aria-hidden="true" />{item}</li>)}
                </ul>
                <p><Bot size={14} aria-hidden="true" />One price per workspace, AI usage included</p>
              </div>
            </InViewOnce>
          </div>
        </section>

        <section id="comparisons">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="Full comparisons" title="Pick the tool you’re weighing up." lede="Each comparison covers features, pricing with a calculator, hosting, switching, and where the other tool is the better fit." /></div>
            <div className="cmp-card-grid">{COMPETITORS.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
          </div>
        </section>

        <section id="method" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="How we compare" title="Written to help you decide." lede="A comparison is only useful if you can trust it, including the parts that don’t favor us." /></div>
            <div className="platform-feature-grid">
              <article><FileSearch size={24} aria-hidden="true" /><h3>Checked against their own pages.</h3><p>Prices and features come from each product’s public pricing pages and documentation, with the date we checked them.</p></article>
              <article><Scale size={24} aria-hidden="true" /><h3>Honest about trade-offs.</h3><p>Every page says where the other tool is stronger today, from channels and mobile SDKs to maturity.</p></article>
              <article><RefreshCw size={24} aria-hidden="true" /><h3>Corrected when things change.</h3><p>Products change quickly. If something is out of date, email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a> and we’ll fix it.</p></article>
            </div>
          </div>
        </section>

        <section id="questions">
          <div className="wrap platform-faq-grid">
            <SectionHead eyebrow="Questions" title="Choosing between Helpin and other tools." />
            <div><FAQList items={FAQS} className="platform-faqs" /></div>
          </div>
        </section>

        <PlatformClosing id="compare-hub-final-title" eyebrow="14-day free trial" title="See the difference in your own workspace." description="Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free." primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
