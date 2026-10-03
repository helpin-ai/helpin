import Link from 'next/link';
import { ArrowRight, BookOpen, BookOpenCheck, Bot, CalendarCheck, Check, Cpu, FileSearch, FolderKanban, GitBranch, Handshake, Inbox, Mic, PenLine, RefreshCw, Scale, Wallet } from 'lucide-react';
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

// Lead with the work Helpin enables; individual pages cover product-specific requirements.
const ADVANTAGES = [
  { Icon: Wallet, title: 'Bring the whole team into the work.', body: 'One workspace price includes unlimited teammates and an AI allowance. Support also has no per-resolution fee.' },
  { Icon: Cpu, title: 'Give agents a job they can finish.', body: 'Use built-in agents, describe a custom one, or connect external agents through MCP. Choose their tools, skills and approvals.' },
  { Icon: BookOpenCheck, title: 'Keep your guides current as you ship.', body: 'Quill can update help articles and internal docs from releases and unanswered questions, with screenshots and browser recordings.' },
  { Icon: RefreshCw, title: 'Follow through after the fix ships.', body: 'After a confirmed release, configured automation can update affected customers under your approval rules, in the original conversation.' },
];

const CHECKED = formatChecked(COMPETITORS[0].checked);
const MATRIX_COLUMNS = MATRIX_TOOLS.map(slug => COMPETITORS.find(item => item.slug === slug)!).map(({ slug, name, category }) => ({ slug, name, category }));

const REASONS = [
  {
    eyebrow: 'Support and product work',
    title: 'Answer the question and work on the problem.',
    body: 'Echo can use your docs and connected tools to check account details, code and logs before replying. When the issue needs a fix, agents can create a task and work on the code. Your team keeps the conversation, investigation and review in view.',
    link: { href: '/products/customer-support', label: 'Explore Support' },
    preview: { product: 'inbox', label: 'The shared inbox with the earlier conversation, linked task and AI draft in view.' },
  },
  {
    eyebrow: 'Projects',
    title: 'AI agents do the work. Your team reviews it.',
    body: 'Move quickly through roadmaps, objectives, epics and sprints in a fast workspace. Agents can plan, write code and review using the task, customer conversation and connected tools. Your team can follow the progress and approve the changes.',
    link: { href: '/products/projects', label: 'Explore Projects' },
    preview: { product: 'projects', label: 'A task with the customer conversation behind it and the agent’s progress.' },
  },
  {
    eyebrow: 'CRM and meetings',
    title: 'Turn customer context into the next sales step.',
    body: 'Beacon can spot buying intent, suggest the next action and carry out approved follow-ups. Deals, conversations and recorded meetings give it the context. Meeting decisions can become assigned tasks, so your team knows what was promised and what to do next.',
    link: { href: '/products/crm', label: 'Explore CRM' },
    preview: { product: 'crm', label: 'A deal with the conversations, meetings and work behind it.' },
  },
] as const;

const TOOLS = [
  { Icon: Inbox, name: 'Help desk', examples: 'Like Intercom, Zendesk or Help Scout' },
  { Icon: FolderKanban, name: 'Issue tracker', examples: 'Like Linear, Jira or Plane' },
  { Icon: Handshake, name: 'CRM', examples: 'Contacts, companies and deals' },
  { Icon: Mic, name: 'Meeting notetaker', examples: 'Calls, decisions and action items' },
  { Icon: BookOpen, name: 'Knowledge base', examples: 'Help center and product docs' },
];

const FAQS = [
  ['Why choose Helpin?', 'Helpin gives your team a fast workspace where AI agents can answer customers, investigate bugs, plan and code changes, update docs and follow up. Support, projects, CRM and meetings share the context, and inviting another teammate does not add a seat fee.'],
  ['Which tools can Helpin replace?', 'Helpin covers a shared inbox and help center, project planning, a CRM with deals, meeting notes and AI agents. It can take the place of a separate help desk, issue tracker, CRM and meeting notetaker, or work alongside the tools you keep through MCP.'],
  ['What should we check before switching?', 'Start with your support channels, mobile SDKs, company sign-in requirements and historical data. Helpin supports web chat and email, web SDKs and selected importers. Each comparison identifies specific differences, such as phone support, SAML sign-in or response-time policies.'],
  ['How much does Helpin cost?', 'Helpin charges one price per workspace with unlimited teammates: Starter is $79 and Growth $239 a month billed annually, each with an AI usage allowance included. The self-hosted Community edition is free.'],
  ['Can I bring my data from another tool?', 'Helpin imports Help Scout Docs articles, Shortcut projects and CRM contacts from CSV today. Importers for Zendesk and Intercom are coming soon, and you can run Helpin alongside your current tool while you move.'],
  ['Is Helpin open source?', 'Yes. Every product feature is open source under AGPL-3.0, including AI agents. Run the Community edition with Docker Compose and your own AI provider, or choose Helpin Cloud.'],
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
      <PreviewNav tone="dark" />
      <div className="platform-page compare-page">
        <JsonLd data={HUB_JSON_LD} />
        <section className="platform-hero motion-hero cmp-hub-hero">
          <HeroVortex variant="connections" tone="dark" />
          <div className="wrap">

            <div className="platform-hero-copy">
              <span className="eyebrow">Compare Helpin</span>
              <h1>All your customer context in one place.<br /><span>No per-seat fees.</span></h1>
              <p className="lede">Give AI agents the work and keep your team in control. Helpin brings support, development, sales, meetings and docs into one fast workspace. See how the complete workflow compares with the tools you use today.</p>
              <CtaRow primaryLabel="Start free trial" primaryHref={SIGNUP_URL} secondaryHref={DEMO_URL} secondaryLabel="Talk to us about switching" />
              <CtaNote trial support />
              <div className="platform-hero-points">
                <span><GitBranch size={14} aria-hidden="true" />Open source · Cloud or self-hosted</span>
                <span><PenLine size={14} aria-hidden="true" />By the Helpin team</span>
                <span><CalendarCheck size={14} aria-hidden="true" />Verified {CHECKED}</span>
              </div>
            </div>
          </div>
        </section>

        <section className="cmp-proof" aria-label="Who uses Helpin"><div className="wrap"><CustomerLogos /></div></section>

        <section id="compare-tools">
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Side by side" title="Compare the work your team can get done." lede="See what is built in, what needs another product, and how agents and pricing fit together." /></div>
            <CompareMatrix tools={MATRIX_COLUMNS} />
            <p className="cmp-mx-note">Verified {CHECKED} from each product’s public pricing and documentation. Open a full comparison for prices, details and trade-offs.</p>
          </div>
        </section>

        <section id="why-teams-switch" className="platform-soft">
          <div className="wrap">
            <div className="platform-centered"><SectionHead eyebrow="Why choose Helpin" title="AI agents follow up, fix bugs, and update docs." lede="Give agents the customer history, tools and instructions to do the work. Your team decides what they can do on their own and what needs review." /></div>
            <ul className="cmp-advantages">
              {ADVANTAGES.map(({ Icon, title, body }) => <li key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></li>)}
            </ul>
            <div className="cmp-reason-rows">
              {REASONS.map((reason, index) => (
                <article key={reason.title} className={index % 2 ? 'cmp-reason-row cmp-reason-row-flip' : 'cmp-reason-row'}>
                  <div className="cmp-reason-copy">
                    <span className="eyebrow">{reason.eyebrow}</span>
                    <h3>{reason.title}</h3>
                    <p>{reason.body}</p>
                    <Link prefetch={false} className="platform-text-link" href={reason.link.href}>{reason.link.label}<ArrowRight size={15} aria-hidden="true" /></Link>
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
            <div className="platform-centered"><SectionHead eyebrow="One customer history" title="Bring the work together, at your pace." lede="Move the tools you juggle into Helpin, or connect the ones you keep. Either way, the history stays in one place." /></div>
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
            <div className="platform-section-intro"><SectionHead eyebrow="Full comparisons" title="Pick the tool you’re weighing up." lede="Compare AI agents, everyday workflows, pricing, hosting and what it takes to switch." /></div>
            <div className="cmp-card-grid">{COMPETITORS.map(item => <CompareCard key={item.slug} competitor={item} />)}</div>
          </div>
        </section>

        <section id="method" className="platform-soft">
          <div className="wrap">
            <div className="platform-section-intro"><SectionHead eyebrow="How we compare" title="Specific features. Real workflows. Clear prices." lede="Compare the jobs your team needs to get done and the tools it takes to do them." /></div>
            <div className="platform-feature-grid">
              <article><FileSearch size={24} aria-hidden="true" /><h3>Checked against their own pages.</h3><p>Prices and features come from each product’s public pricing pages and documentation, with the date we checked them.</p></article>
              <article><Scale size={24} aria-hidden="true" /><h3>Details that help you choose.</h3><p>Each page explains the workflow, plan requirements and specific differences, including channels, mobile SDKs and imports.</p></article>
              <article><RefreshCw size={24} aria-hidden="true" /><h3>Corrected when things change.</h3><p>Products change quickly. If something is out of date, email <a href="mailto:hello@helpin.ai">hello@helpin.ai</a> and we’ll fix it.</p></article>
            </div>
          </div>
        </section>

        <section id="questions">
          <div className="wrap platform-faq-grid">
            <SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s alternatives" />
            <div><FAQList items={FAQS} className="platform-faqs" /></div>
          </div>
        </section>

        <PlatformClosing id="compare-hub-final-title" eyebrow="14-day free trial" title="See the difference in your own workspace." description="Start a 14-day trial of Helpin Cloud with no card, or self-host the open-source edition for free." primaryLabel="Start free trial" primaryHref={SIGNUP_URL} />
      </div>
      <PreviewFooter />
    </>
  );
}
