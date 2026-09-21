import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, ChevronRight, Code2, FolderOpen, History, Languages, Link2, ShieldCheck, Search, Server, Network, Braces } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { SupportKnowledge } from '../customer-support/support-knowledge';
import { KnowledgeScene } from './knowledge-scenes';
import './knowledge-publishing.css';
import { PublishingDemoV2 } from './publishing-demo-v2';
import './knowledge.css';
import { KnowledgeWorkspace, KnowledgeReader, KnowledgeAPI } from './knowledge-previews';

export const metadata = previewMetadata("Knowledge \u2014 Helpin", "/new/products/knowledge");
const FEATURES = [
  { Icon: FolderOpen, title: 'Make the next step easy to find.', body: 'Organize guides into nested collections, with article navigation and a table of contents for longer answers.' },
  { Icon: Search, title: 'Answer the question behind the search.', body: 'Let readers find a guide or ask in their own words. Enable AI answers with article citations, so customers can follow the answer back to its source.' },
  { Icon: Languages, title: 'Publish for every audience.', body: 'Manage translated articles and collections, with language selection and separate review and publishing states.' },
];
const AUTHORING = [
  { Icon: Code2, title: 'Check the source behind the claim.', body: 'Give the docs agent the relevant code and product history. It can inspect the implementation and tests behind a feature before proposing what the docs should say.' },
  { Icon: BookOpen, title: 'Improve the guide you already have.', body: 'Find the article that needs a correction, add a missing step, or draft a guide for something new. Keep the answer in the right place.' },
  { Icon: History, title: 'Review the change with its reasons.', body: 'See the proposed edits and the evidence behind them. Keep draft changes separate from the live article, with document history to follow what changed.' },
];
const FAQS = [
  [
    "Can we serve docs from our own website?",
    "Yes. Use a Helpin address, your own domain, or a path on your existing site.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "Are help-center pages fast to load?",
    "Article content is delivered in the initial page response, with caching and compression available to speed up delivery.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "What is included for search engines?",
    "Helpin generates page metadata, sitemaps and language links, and supports redirects when articles move.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "Can developers try our API from the docs?",
    "Yes. Import your API definition to create an interactive reference with code examples and a request panel.",
    "/new/products/knowledge#knowledge-api"
  ],
  [
    "Does the help center include AI answers?",
    "Yes. Readers can ask questions and get answers with article citations, or the closest articles when the docs don’t answer confidently.",
    "/new/products/knowledge#knowledge-library"
  ],
  [
    "Can we keep internal documentation too?",
    "Yes. Keep spaces internal or restrict them to selected teams; public publishing and agent knowledge sources are separate settings.",
    "/new/products/knowledge#knowledge-sources"
  ],
  [
    "What knowledge can agents use?",
    "Select public docs, website pages and supported files. Choose a whole space, a collection or individual articles.",
    "/new/products/knowledge#knowledge-sources"
  ],
  [
    "How does Helpin find missing knowledge?",
    "Unanswered questions and weak answers reveal coverage gaps. Your team can investigate and prepare a draft article or update.",
    "/new/products/knowledge#knowledge-gaps"
  ],
  [
    "What does the docs agent do?",
    "The docs agent investigates support gaps, checks existing guides and prepares updates from the available evidence. Your team can review changes before publishing.",
    "/new/products/knowledge#knowledge-quill"
  ],
  [
    "Can we self-host Knowledge?",
    "Yes. Docs is in the free Community edition.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ]
] as const;
export default function KnowledgePage() {
  return <><PreviewNav /><main className="knowledge-page">
    <section className="knowledge-hero" aria-labelledby="knowledge-title"><div className="wrap">
      <div className="knowledge-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Knowledge</span></div>
      <div className="knowledge-hero-copy"><Availability category="Help center & docs" /><h1 id="knowledge-title">Give customers and agents <span>docs they can rely on.</span></h1><p className="lede">Publish guides and API references on your own domain. Help customers find answers, give agents knowledge to work from, and let the docs agent prepare updates as your product changes.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /><div className="knowledge-hero-points"><span><Server size={14} />On your own domain</span><span><Network size={14} />Fast to load</span><span><Code2 size={14} />Developer docs included</span><span><Link2 size={14} />AI answers with sources</span></div></div>
      <div className="knowledge-screenshot"><KnowledgeWorkspace /></div>
    </div></section>
    <nav className="knowledge-page-nav" aria-label="On this page"><div className="wrap"><strong>Knowledge</strong><a href="#knowledge-library">Reader experience</a><a href="#knowledge-publishing">Publishing & performance</a><a href="#knowledge-api">Developer docs</a><a href="#knowledge-sources">Agent knowledge</a><a href="#knowledge-quill">Keep docs current</a></div></nav>
    <section id="knowledge-library"><div className="wrap"><SectionHead eyebrow="The experience your customers see" title="Make your docs feel like part of your product." lede="Give readers a clear path from their first question to the next step. Put searchable guides, source-backed AI answers, and your brand into one help center." /><div className="knowledge-reader-image"><KnowledgeReader /></div><div className="knowledge-features">{FEATURES.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-publishing" className="knowledge-publishing-section"><div className="wrap"><SectionHead eyebrow="Built to be part of your website" title="Your docs belong on your domain." lede="Use a Helpin address, connect your own domain, or serve docs under /docs through a reverse proxy — a connection that serves Helpin docs from your existing website. Keep the address, branding, and reading experience yours." /><PublishingDemoV2 /><div className="knowledge-delivery-details"><article><span className="knowledge-detail-icon"><Server size={21}/></span><div><h3>Deliver the article, ready to read.</h3><p>Server-side rendering puts article content in the first HTML response. Caching and compression keep the delivery efficient as readers move through your docs.</p><div className="knowledge-tech-points"><span>Server-rendered HTML</span><span>HTML caching</span><span>Response compression</span></div></div></article><article><span className="knowledge-detail-icon"><Search size={21}/></span><div><h3>Give search engines a clear path.</h3><p>Canonical URLs, sitemaps, structured metadata, and language-alternate links help search engines understand your docs—even when they live behind your proxy.</p><div className="knowledge-tech-points"><span>Canonical URLs</span><span>Sitemaps & redirects</span><span>Social previews</span></div></div></article></div><p className="knowledge-proxy-platforms">Setup guidance for <span>Cloudflare Workers</span><span>AWS CloudFront</span><span>Vercel</span></p></div></section>
    <section id="knowledge-api"><div className="wrap"><SectionHead eyebrow="For the people building with your product" title="An API reference developers can try." lede="Turn an OpenAPI specification — a file describing your API — into an interactive reference beside your guides. Let developers explore endpoints, switch code examples, and try requests against your API without leaving the docs." /><div className="knowledge-reader-image knowledge-api-image"><KnowledgeAPI /></div><div className="knowledge-api-points"><span><Braces size={15}/>cURL, JavaScript & Python</span><span><Code2 size={15}/>Import from a URL or file</span><span><ShieldCheck size={15}/>Authentication-aware request playground</span></div></div></section>
    <section id="knowledge-sources" className="knowledge-sources-section"><div className="wrap knowledge-split"><SectionHead eyebrow="The knowledge behind the answer" title="The same docs power your AI answers." lede="Give agents the same guides your customers read. Connect public Helpin docs, website pages, and files, then choose which sources each agent can use." secondaryLede="Select a whole docs space, a collection, or individual articles. Check source status and refresh the index when the content changes." /><KnowledgeScene variant="sources" /></div></section>
    <section id="knowledge-gaps" className="knowledge-gaps-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="Let support show you what to write" title="See where your docs leave customers stuck." lede="Repeated questions. Incomplete answers. A guide that no longer matches the product. Helpin connects knowledge gaps to the conversations behind them, so your team knows what needs attention." secondaryLede="Turn that evidence into a new article or a focused update. Review the draft before it becomes the next customer’s answer." /><Link className="knowledge-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><SupportKnowledge /></div></section>
    <section id="knowledge-quill" className="knowledge-quill-section"><div className="wrap"><div className="knowledge-split"><div><span className="knowledge-agent-label"><img src="/new/agents/quill.svg" width={32} height={32} alt="" />Meet the docs agent</span><SectionHead eyebrow="From shipped changes to useful docs" title="The docs agent updates guides when the product changes." lede="A setup step changed. An endpoint behaves differently. A new feature needs a guide. Ask the docs agent to check the source, find the affected docs, and prepare the update." secondaryLede="Give it the relevant docs, customer evidence, and code access. Review what changed and why before you publish." /><Link className="knowledge-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><KnowledgeScene variant="quill" /></div><div className="knowledge-features knowledge-authoring">{AUTHORING.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true"/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-faq"><div className="wrap knowledge-faq-grid"><SectionHead eyebrow="Questions" title="What to know before you publish." /><div className="knowledge-faqs"><FAQList items={FAQS} className="faq-items" /></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="knowledge-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Make the next answer better</span><h2 id="knowledge-final-title">Make your next release easier to understand.</h2><p className="lede">Publish the guide, answer the question, and keep the knowledge connected to what your team ships.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /></div></div></section>
  </main><PreviewFooter /></>;
}
