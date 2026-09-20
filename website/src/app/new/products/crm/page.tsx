import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, ArrowUpRight, Building2, ChevronRight, FileInput, ListFilter, Mail, Network, SlidersHorizontal, Sparkles } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { CRMAccountScene, CRMPipeline, CRMSignals, CRMPlaybook } from './crm-scenes';
import './crm.css';

export const metadata: Metadata = {
  title: 'CRM — Helpin',
  description: 'Know the customer. See the next move. Connect contacts, companies, deals, and customer signals to the conversations and work behind every relationship.',
  alternates: { canonical: '/new/products/crm' },
  robots: { index: false, follow: false },
};

const ESSENTIALS = [
  { Icon: Building2, title: 'Give every relationship a home.', body: 'Organize contacts and companies with owners, lifecycle stages, labels, and the records they belong to.' },
  { Icon: Mail, title: 'Keep the conversation close.', body: 'Connect Gmail and Google Calendar so email threads and meetings can sit alongside your customer records.' },
  { Icon: SlidersHorizontal, title: 'Track what matters to your team.', body: 'Add custom properties and configure pipeline stages to reflect how you sell and support customers.' },
  { Icon: ListFilter, title: 'Find the accounts you need.', body: 'Search and filter contacts, companies, and deals. Switch between a pipeline board and a detailed deal list.' },
  { Icon: FileInput, title: 'Bring your existing relationships.', body: 'Import contacts, companies, and deals from CSV. Map columns to fields before processing your import.' },
  { Icon: Sparkles, title: 'Fill in the missing context.', body: 'Use enrichment to research customer and company details, with source links you can check.' },
];
const FAQS = [
  ['What can I manage in Helpin CRM?', 'Contacts, companies, deals, pipelines, email conversations, meetings, and customer activity. Customer records connect to support conversations and project work, so the people managing a relationship can see what the rest of the team is doing.'],
  ['Can I manage renewals and expansion as well as new business?', 'Yes. Deals support new and existing business, and pipelines can reflect different commercial motions, including conversion, expansion, and renewal. Configure stages, assign owners, and track amounts, probabilities, and expected close dates.'],
  ['Where do customer signals come from?', 'Helpin combines evidence from conversations and connected customer activity to surface buying intent, objections, risks, and other changes. You can inspect the evidence and proposed next step. Which signals appear and trigger downstream work depends on the rules and connections enabled for your workspace.'],
  ['Will agents send messages or change deals without review?', 'In connected CRM playbooks, proposed customer messages, CRM changes, and project tasks require approval or can be disallowed. Connecting or publishing a playbook does not start automation. Your team separately enables it and chooses how qualifying signals enter.'],
  ['Can we bring our existing data and email?', 'CSV imports support contacts, companies, and deals with field mapping. Gmail and Google Calendar connections bring customer conversations and events into the workspace. You can also work with CRM records through the API.'],
  ['Is CRM included in the Community beta?', 'The current Community beta focuses on support, docs, and agents. The full CRM, Meetings, and Projects workflows shown on this page depend on your edition and configured services.'],
];

export default function CRMPage() {
  return <><PreviewNav /><div className="crm-page">
    <section className="crm-hero" aria-labelledby="crm-title"><div className="wrap">
      <div className="crm-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>CRM</span></div>
      <div className="crm-hero-grid"><div className="crm-hero-copy"><span className="eyebrow">CRM, with the customer context</span><h1 id="crm-title">Know the customer.<br /><span>See the next move.</span></h1><p className="lede">Bring the deal, the conversation, and the work into one view. Know what’s holding a customer back—and give your team and agents the context to move things forward.</p><CtaRow secondaryHref="#crm-pipeline" secondaryLabel="Explore CRM" /><p className="crm-hero-note">From the first conversation to the next renewal.</p></div><CRMAccountScene /></div>
      <div className="crm-outcomes">{[['01', 'Know the account', 'One history across teams.'], ['02', 'Find the next step', 'Evidence behind every signal.'], ['03', 'Follow through', 'An owner and work attached.']].map(([n, title, body]) => <div key={n}><span>{n}</span><div><strong>{title}</strong><p>{body}</p></div></div>)}</div>
    </div></section>

    <section id="crm-record"><div className="wrap">
      <div className="crm-section-intro"><SectionHead eyebrow="The relationship, in context" title="Walk into every conversation with the full story." lede="The last email. The rollout discussion. The support issue still waiting on a fix. Keep them connected to the contact, company, and deal—so the next person picks up where the last one left off." /><div className="crm-section-aside"><Network size={23} /><p>One customer history.<br />Shared by your team and agents.</p></div></div>
      <figure className="crm-product-image"><a href="/new/product/workspace-crm-4k-v3.webp" target="_blank" rel="noopener noreferrer" aria-label="Open the illustrative CRM screenshot at full size (new tab)"><img src="/new/product/workspace-crm-1920-v3.webp" srcSet="/new/product/workspace-crm-960-v3.webp 960w, /new/product/workspace-crm-1920-v3.webp 1920w, /new/product/workspace-crm-4k-v3.webp 3840w" sizes="(max-width: 1280px) calc(100vw - 48px), 1232px" width={3840} height={2160} loading="lazy" alt="Illustrative OrbitDesk CRM record for Maya Chen at Northstar Labs, showing a customer summary, buying signal, emails, meetings, support activity, and linked work." /></a><figcaption><span>One record. The people, conversations, and work behind it.</span><a href="/new/product/workspace-crm-4k-v3.webp" target="_blank" rel="noopener noreferrer">Illustrative product view <ArrowUpRight size={13} /></a></figcaption></figure>
      <div className="crm-record-benefits"><article><h3>Get up to speed.</h3><p>Read the customer summary and recent activity before the next call. See the history that explains the current situation.</p></article><article><h3>Keep the handoff intact.</h3><p>Link the people, companies, deals, and tasks involved. Give sales, support, and product a shared starting point.</p></article><article><h3>Follow the work.</h3><p>Find the customer request behind a task and the deal it affects. Keep the relationship connected as the work progresses.</p></article></div>
    </div></section>

    <section id="crm-pipeline" className="crm-soft"><div className="wrap"><div className="crm-centered"><SectionHead eyebrow="A pipeline you can act on" title="See what’s holding up the deal." lede="Track stages, owners, and close dates. Then look beyond the amount: the customer’s concern, the work in progress, and the next step that could move the relationship forward." /></div><CRMPipeline /><div className="crm-under-demo"><span>Configurable pipelines & stages</span><span>Board & list views</span><span>Owners, amounts & close dates</span></div></div></section>

    <section id="crm-signals" className="crm-dark"><div className="wrap"><div className="crm-section-intro"><SectionHead eyebrow="Know where to focus" title="Catch the change. Understand the reason." lede="Buying intent in a support reply. A concern ahead of renewal. A request to bring in another team. Surface the signal, check the evidence, and agree the next step." /><span className="crm-small-label">CUSTOMER EVIDENCE → NEXT ACTION</span></div><CRMSignals /><p className="crm-dark-note">Your customer’s words stay attached to the recommendation.</p></div></section>

    <section id="crm-follow-through"><div className="wrap crm-split"><div><SectionHead eyebrow="From a signal to a customer outcome" title="Give the next step an owner. And a way forward." lede="Use playbooks to define the outcome, milestones, and responsibilities behind a follow-up, sales handoff, or renewal recovery." secondaryLede="Connect an agent to review updates and propose the work. Your team approves customer messages, CRM changes, and tasks before those playbook actions run." /><Link className="crm-text-link" href="/new/products/ai-agents">Meet the agents behind the work <ArrowRight size={16} /></Link></div><CRMPlaybook /></div></section>

    <section className="crm-essentials crm-soft"><div className="wrap"><SectionHead eyebrow="The everyday CRM, covered" title="A practical home for your customer relationships." /><div className="crm-feature-grid">{ESSENTIALS.map(({ Icon, title, body }) => <article key={title}><Icon size={22} strokeWidth={1.6} /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>

    <section id="crm-faq"><div className="wrap crm-faq-grid"><SectionHead eyebrow="A few useful answers" title="Get to know your CRM." /><div className="crm-faqs">{FAQS.map(([q, a]) => <details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="crm-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Keep the relationship moving</span><h2 id="crm-final-title">Every customer has a next chapter.</h2><p className="lede">Bring the context, the people, and the work together to help it happen.</p><CtaRow /></div></div></section>
  </div><PreviewFooter /></>;
}
