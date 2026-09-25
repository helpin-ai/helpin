import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { marketingMetadata } from '../../_components/marketing-metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, ChevronRight, Code2, FolderOpen, History, Languages, Link2, ShieldCheck, Search, Server, Braces } from 'lucide-react';
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

export const metadata = marketingMetadata("Knowledge \u2014 Helpin", "/products/knowledge");
const FEATURES = [
  { Icon: FolderOpen, title: 'Find the right guide.', body: 'Organize articles into collections. Give longer guides a table of contents so readers can jump to the part they need.' },
  { Icon: Search, title: 'Ask in your own words.', body: 'Enable AI answers with article citations, so readers can check the guidance behind the response.' },
  { Icon: Languages, title: 'Help customers in their language.', body: 'Translate the help center, collections, and articles. Each language is reviewed and published on its own.' },
];
const AUTHORING = [
  { Icon: Code2, title: 'Check what the product actually does.', body: 'Give the agent access to relevant implementation details and tests before it drafts the explanation.' },
  { Icon: BookOpen, title: 'Improve the right article.', body: 'Correct an existing guide, add a missing step, or prepare a new article when the topic needs its own explanation.' },
  { Icon: History, title: 'Review the edit and its reasons.', body: 'Check the proposed change before publishing. Keep the draft separate from the live article and use document history to follow the edits.' },
];
const FAQS = [
  [
    "Can we review AI-written content before publishing?",
    "Yes. Agents prepare drafts and proposed edits. Only your team publishes to the help center, so nothing an agent writes goes live on its own.",
    "/products/knowledge#knowledge-quill"
  ],
  [
    "Can agents use our internal docs?",
    "Yes. Choose any Helpin space, public or internal, then narrow it to a collection or individual articles. Spaces can also be limited to selected teams.",
    "/products/knowledge#knowledge-sources"
  ],
  [
    "Which files can agents use?",
    "PDF, DOCX, Markdown, plain text, CSV, and JSON. Website pages are crawled from your site, with include and exclude rules.",
    "/products/knowledge#knowledge-sources"
  ],
  [
    "Where do test requests go?",
    "Requests from the API reference run in the reader’s browser and go straight to your API, so it needs to allow requests from your docs address.",
    "/products/knowledge#knowledge-api"
  ],
  [
    "Which Knowledge features need the Growth plan?",
    "On Cloud, Starter includes 500 documents. Growth adds unlimited documents, a multilingual help center, and AI article translation. All of it is included when you self-host.",
    "/pricing",
    "Compare plans"
  ],
  [
    "Can we import docs from Zendesk or Intercom?",
    "Zendesk and Intercom import is coming soon."
  ],
  [
    "Can we self-host Knowledge?",
    "Yes. Community includes every module by default, including docs and the help center. It is free under AGPL-3.0 with no plan limits. Community is currently a 0.1 beta.",
    "/self-hosting#whats-included",
    "See what’s included"
  ]
] as const;
export default function KnowledgePage() {
  return <><PreviewNav /><main className="knowledge-page">
    <section className="knowledge-hero motion-hero" aria-labelledby="knowledge-title"><HeroVortex variant="flow" tone="dark" /><div className="wrap">
      <div className="knowledge-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} /><span>Knowledge</span></div>
      <div className="knowledge-hero-copy"><span className="eyebrow">Help center, product docs, and team knowledge</span><h1 id="knowledge-title">Better docs for your customers.<br /><span>Better answers from your AI agents.</span></h1><p className="lede">Publish help articles, product guides, and API docs in one place. Agents draft updates from support gaps and shipped changes. Nothing goes live until your team publishes it.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day free trial · No card required</p><div className="knowledge-hero-points"><span><Server size={14} />Your own domain</span><span><Search size={14} />Search and AI answers</span><span><Code2 size={14} />Interactive API docs</span><span><ShieldCheck size={14} />Your team publishes</span></div></div>
      <div className="knowledge-screenshot"><KnowledgeWorkspace /><p className="knowledge-demo-caption">Collections, articles, and the API reference in one space.</p></div>
    </div></section>
    <nav className="knowledge-page-nav" aria-label="On this page"><div className="wrap"><strong>Knowledge</strong><a href="#knowledge-library">Reader experience</a><a href="#knowledge-publishing">Publishing & performance</a><a href="#knowledge-api">Developer docs</a><a href="#knowledge-sources">Agent knowledge</a><a href="#knowledge-gaps">Support gaps</a><a href="#knowledge-quill">Keep docs current</a></div></nav>
    <section id="knowledge-library"><div className="wrap"><SectionHead eyebrow="Help center" title="Browse, search, or just ask." lede="Give customers a help center they can browse, search, or ask a question. AI answers cite the articles they come from, and when the docs can’t answer confidently, readers see the relevant articles instead." /><div className="knowledge-reader-image"><KnowledgeReader /><p className="knowledge-demo-caption">A table of contents, search, and AI answers on every article.</p></div><div className="knowledge-features">{FEATURES.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div><p className="knowledge-supporting-note">On Cloud, a multilingual help center and AI article translation are on the Growth plan. Both are included when you self-host.</p></div></section>
    <section id="knowledge-publishing" className="knowledge-publishing-section section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><SectionHead eyebrow="Your help center. Your website." title="Keep your docs where customers expect them." lede="Use a Helpin address, connect your domain, or publish under a path such as /docs on your existing website through a reverse proxy." /><PublishingDemoV2 /><div className="knowledge-delivery-details"><article><span className="knowledge-detail-icon"><Server size={21}/></span><div><h3>Articles arrive in the first HTML response.</h3><p>Pages are server-rendered, cached, and compressed with Brotli or gzip.</p><div className="knowledge-tech-points"><span>Server-rendered HTML</span><span>HTML caching</span><span>Response compression</span></div></div></article><article><span className="knowledge-detail-icon"><Search size={21}/></span><div><h3>Help search engines find your guides.</h3><p>Use metadata, canonical URLs, sitemaps, language links, and redirects to keep your published content discoverable.</p><div className="knowledge-tech-points"><span>Canonical URLs</span><span>Sitemaps & redirects</span><span>Social previews</span></div></div></article></div><p className="knowledge-proxy-platforms">Setup guidance for <span>Cloudflare Workers</span><span>AWS CloudFront</span><span>Vercel</span><span>nginx</span></p></div></section>
    <section id="knowledge-api"><div className="wrap"><SectionHead eyebrow="For the people building with your product" title="Help developers go from reading to making a request." lede="Import an OpenAPI specification to publish an interactive reference alongside your guides. Show developers how to authenticate, pass parameters, and try your API." /><div className="knowledge-reader-image knowledge-api-image"><KnowledgeAPI /><p className="knowledge-demo-caption">The endpoint, the example, and the request—in one place.</p></div><div className="knowledge-api-points"><span><Braces size={15}/>Switch code examples</span><span><Code2 size={15}/>Import an OpenAPI URL or file</span><span><ShieldCheck size={15}/>Try requests from the reference</span></div></div></section>
    <section id="knowledge-sources" className="knowledge-sources-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="The knowledge behind the answer" title="Give your agents the guidance you want them to use." lede="Choose the Helpin docs (public or internal), website pages, and files each agent can use: PDF, DOCX, Markdown, plain text, CSV, or JSON. Select a whole space, a collection, or individual articles, and see when each source is ready. The index refreshes automatically when you publish or unpublish." /><div className="knowledge-source-choice"><h3>Public guides and internal docs have different jobs.</h3><p>Keep customer-facing guidance public and team playbooks internal. Publishing a document and selecting it as agent knowledge are separate settings.</p></div></div><KnowledgeScene variant="sources" /></div></section>
    <section id="knowledge-gaps" className="knowledge-gaps-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="Let customer questions guide the next update" title="Your next useful article may start in a support conversation." lede="Helpin groups repeated questions into gaps, using AI handoffs, failed help-center searches, article feedback, and human replies. Each gap lists its conversations. Draft a new article or an update, then review it before it goes live." /><Link className="knowledge-inline-link" href="/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><SupportKnowledge variant="knowledge" /></div></section>
    <section id="knowledge-quill" className="knowledge-quill-section"><div className="wrap"><div className="knowledge-split"><div><span className="knowledge-agent-label"><img src="/new/agents/quill.svg" width={32} height={32} alt="" />Meet the docs agent</span><SectionHead eyebrow="Keep the guidance connected to the product" title="The product changed. Help the docs catch up." lede="Ask the docs agent to review the relevant guides, inspect the available product evidence, and prepare an update. Give your team the proposed edit and the reason behind it—not another article to start from scratch." /><Link className="knowledge-inline-link" href="/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><div><KnowledgeScene variant="quill" /><p className="knowledge-demo-caption">Checked against the released change before you review it.</p></div></div><div className="knowledge-features knowledge-authoring">{AUTHORING.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true"/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-faq"><div className="wrap knowledge-faq-grid"><SectionHead eyebrow="Before you publish" title="Get to know Helpin Knowledge." /><div className="knowledge-faqs"><FAQList items={FAQS} className="faq-items" /><Link className="knowledge-inline-link" href="/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="knowledge-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><h2 id="knowledge-final-title">Help the next customer<br />find the answer.</h2><p className="lede">Keep your help center, product docs, and AI knowledge connected to the questions customers ask and the changes your team ships.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day free trial · No card required</p><p className="knowledge-supporting-note">Open source · Self-host free, or let us run it</p></div></div></section>
  </main><PreviewFooter homepage /></>;
}
