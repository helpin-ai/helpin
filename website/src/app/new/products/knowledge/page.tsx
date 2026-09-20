import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, BookOpen, ChevronRight, Code2, FolderOpen, Globe, History, Languages, Link2, ShieldCheck } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { SupportKnowledge } from '../customer-support/support-knowledge';
import { KnowledgeLibrary, KnowledgeScene } from './knowledge-scenes';
import './knowledge.css';

export const metadata: Metadata = {
  title: 'Knowledge — Helpin',
  description: 'Create a help center, organize internal docs, and give agents the sources they need. Turn customer questions and product changes into documentation your team can review.',
  alternates: { canonical: '/new/products/knowledge' },
  robots: { index: false, follow: false },
};
const FEATURES = [
  { Icon: FolderOpen, title: 'Give every guide a home.', body: 'Organize documents into spaces and nested collections so related answers stay together.' },
  { Icon: Globe, title: 'Make your help center your own.', body: 'Set your branding and publish on a Helpin subdomain or your own domain.' },
  { Icon: ShieldCheck, title: 'Keep internal knowledge for your team.', body: 'Use internal spaces with workspace or team visibility. Choose what you publish for customers.' },
  { Icon: History, title: 'Keep track of what changed.', body: 'Review document versions and keep draft changes separate from the published article.' },
  { Icon: Languages, title: 'Help customers in their language.', body: 'Manage translated articles and collections, with review and publishing states for each language.' },
  { Icon: Code2, title: 'Put API guidance beside your docs.', body: 'Import an OpenAPI specification to publish an interactive API reference in your help center.' },
];
const FAQS = [
  ['Can we publish a customer help center?', 'Yes. Create a public documentation space, organize articles into collections, and publish a branded help center. You can use a Helpin subdomain or configure your own domain.'],
  ['Can we keep internal documentation too?', 'Yes. Internal spaces can be visible across the workspace or restricted to selected teams. Public publishing and agent knowledge sources are separate settings; an internal document is not automatically a customer-facing answer.'],
  ['What knowledge can agents use?', 'Select supported public Helpin docs at the space, collection, or article level. You can also add website sources and supported files, including PDF, Markdown, text, and CSV. Review source status and reindex sources when needed.'],
  ['How does Helpin find missing knowledge?', 'Support coverage gaps connect unanswered or weakly answered questions to conversation evidence. Your team can investigate a gap and prepare a new article or an update to an existing one. New suggested articles remain drafts until published.'],
  ['What does Quill do?', 'Quill is the documentation agent. It can investigate support gaps, audit existing docs, and prepare changes using the relevant workspace evidence and tools available to it. Where repository context is connected, it can check implementation details before proposing product documentation.'],
  ['Can we review an AI-written article before it goes live?', 'Yes. Review drafts and proposed changes before publishing. Agent tool access and approval settings determine which actions an agent can take.'],
  ['Can we self-host Knowledge?', 'Yes. Docs is part of the open-source Community beta, alongside support and agents. Connected project, CRM, and repository workflows depend on the edition and tools available in your workspace.'],
];
export default function KnowledgePage() {
  return <><PreviewNav /><main className="knowledge-page">
    <section className="knowledge-hero" aria-labelledby="knowledge-title"><div className="wrap">
      <div className="knowledge-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Knowledge</span></div>
      <div className="knowledge-hero-copy"><span className="eyebrow">Docs, help center & agent knowledge</span><h1 id="knowledge-title">Turn what your team learns <span>into useful answers.</span></h1><p className="lede">Create docs for your customers and your team. Give agents the right sources, find what’s missing, and keep answers connected to the work behind them.</p><CtaRow secondaryHref="#knowledge-library" secondaryLabel="Explore Knowledge" /><div className="knowledge-hero-points"><span><Globe size={14} />Public help center</span><span><BookOpen size={14} />Internal docs</span><span><Link2 size={14} />Agent knowledge</span></div></div>
      <figure className="knowledge-screenshot"><img src="/new/product/workspace-knowledge-1920-v3.webp" srcSet="/new/product/workspace-knowledge-960-v3.webp 960w, /new/product/workspace-knowledge-1920-v3.webp 1920w, /new/product/workspace-knowledge-4k-v3.webp 3840w" sizes="(max-width: 1280px) calc(100vw - 48px), 1232px" width={3840} height={2160} fetchPriority="high" alt="OrbitDesk Help Center in the Knowledge workspace, with collections, published guides, and an API reference." /></figure>
    </div></section>
    <nav className="knowledge-page-nav" aria-label="On this page"><div className="wrap"><strong>Knowledge</strong><a href="#knowledge-library">Docs & help center</a><a href="#knowledge-sources">Agent sources</a><a href="#knowledge-gaps">Coverage gaps</a><a href="#knowledge-quill">Quill</a><a href="#knowledge-faq">FAQs</a></div></nav>
    <section id="knowledge-library"><div className="wrap"><SectionHead eyebrow="A place for what you know" title="Write it once, then put it where it helps." lede="Publish guides customers can find themselves. Keep the processes and playbooks your team relies on in internal spaces. Organize both in the same workspace." /><KnowledgeLibrary /><div className="knowledge-features">{FEATURES.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-sources" className="knowledge-sources-section"><div className="wrap knowledge-split"><SectionHead eyebrow="Give agents something to work from" title="Choose the knowledge behind the answer." lede="Connect public Helpin docs, website pages, and files as knowledge sources. Select a whole docs space, a collection, or individual articles." secondaryLede="See source status and refresh the index when needed. Your team decides which material is available to the agent." /><KnowledgeScene variant="sources" /></div></section>
    <section id="knowledge-gaps"><div className="wrap knowledge-split"><div><SectionHead eyebrow="Learn from the questions" title="Turn missing answers into better documentation." lede="A customer question can reveal a missing guide, an unclear explanation, or an outdated article. Keep the conversation evidence attached as you investigate and prepare an update." secondaryLede="Review the suggested draft before it becomes a published answer." /><Link className="knowledge-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><SupportKnowledge /></div></section>
    <section id="knowledge-quill" className="knowledge-quill-section"><div className="wrap knowledge-split"><div><span className="knowledge-agent-label"><img src="/new/agents/quill.svg" width={32} height={32} alt="" />Meet Quill, your documentation agent</span><SectionHead eyebrow="Keep up with the product" title="Give your docs an agent that checks the details." lede="Ask Quill to investigate a gap, review an existing guide, or prepare an update. It works from the evidence and tools you give it, including connected product and repository context." secondaryLede="Bring the proposed changes back to your team with the reasons behind them." /><Link className="knowledge-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><KnowledgeScene variant="quill" /></div></section>
    <section id="knowledge-faq"><div className="wrap knowledge-faq-grid"><SectionHead eyebrow="A few useful answers" title="Build the knowledge your team can rely on." /><div className="knowledge-faqs">{FAQS.map(([q,a])=><details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="knowledge-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Make the next answer better</span><h2 id="knowledge-final-title">Put what your team knows to work.</h2><p className="lede">Connect your docs, customer questions, and agents in one workspace.</p><CtaRow /></div></div></section>
  </main><PreviewFooter /></>;
}
