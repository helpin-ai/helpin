import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
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

export const metadata = previewMetadata("Knowledge \u2014 Helpin", "/new/products/knowledge");
const FEATURES = [
  { Icon: FolderOpen, title: 'Find the right guide.', body: 'Organize articles into collections. Give longer guides a table of contents so readers can jump to the part they need.' },
  { Icon: Search, title: 'Ask in your own words.', body: 'Enable AI answers with article citations, so readers can check the guidance behind the response.' },
  { Icon: Languages, title: 'Help customers in their language.', body: 'Manage translated articles and collections, with review and publishing handled for each language.' },
];
const AUTHORING = [
  { Icon: Code2, title: 'Check what the product actually does.', body: 'Give the agent access to relevant implementation details and tests before it drafts the explanation.' },
  { Icon: BookOpen, title: 'Improve the right article.', body: 'Correct an existing guide, add a missing step, or prepare a new article when the topic needs its own explanation.' },
  { Icon: History, title: 'Review the edit and its reasons.', body: 'Check the proposed change before publishing. Keep the draft separate from the live article and use document history to follow the edits.' },
];
const FAQS = [
  [
    "Can we publish docs on our own website?",
    "Yes. Use a Helpin address, connect a custom domain, or serve docs from a path on your existing website through a reverse proxy.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "How are help-center pages delivered?",
    "Article content is server-rendered, with caching and compression supporting delivery. Readers receive the article in the initial HTML response.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "What is included for search engines?",
    "Page metadata, canonical URLs, sitemaps, language links, and redirects help search engines find and understand your published docs.",
    "/new/products/knowledge#knowledge-publishing"
  ],
  [
    "Can developers try our API from the docs?",
    "Yes. Import an OpenAPI specification to create an interactive reference with authentication details, code examples, and a request panel.",
    "/new/products/knowledge#knowledge-api"
  ],
  [
    "Does the help center offer AI answers?",
    "Yes. Enable answers with article citations. When the available guidance does not support a confident answer, readers can be shown relevant articles instead.",
    "/new/products/knowledge#knowledge-library"
  ],
  [
    "Can we keep internal documentation too?",
    "Yes. Keep spaces internal or limit them to selected teams. Public publishing and agent knowledge sources are configured separately.",
    "/new/products/knowledge#knowledge-sources"
  ],
  [
    "Which sources can agents use?",
    "Select public Helpin docs, website pages, and supported files. For docs, choose a space, collection, or individual articles.",
    "/new/products/knowledge#knowledge-sources"
  ],
  [
    "How do we find gaps in our documentation?",
    "Review unanswered questions and weak answers alongside their source conversations. Use the evidence to decide whether to improve an existing guide or create something new.",
    "/new/products/knowledge#knowledge-gaps"
  ],
  [
    "What does the docs agent help with?",
    "It helps create and maintain help articles, internal documentation, and API docs. Give it the relevant context and tools, then review the proposed work according to your approval settings.",
    "/new/products/knowledge#knowledge-quill"
  ],
  [
    "Can we self-host Knowledge?",
    "Yes. Knowledge and the help center are included in the open-source product. Your team operates the installation and covers hosting and provider costs. Enterprise features are licensed separately.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ],
  ["Can we review AI-written content before publishing?", "Yes. Review drafts and proposed edits before they go live. Configure which agent actions require approval rather than treating every generated update as ready to publish.", "/new/products/knowledge#knowledge-quill"]
] as const;
export default function KnowledgePage() {
  return <><PreviewNav /><main className="knowledge-page">
    <section className="knowledge-hero motion-hero" aria-labelledby="knowledge-title"><HeroVortex variant="flow" tone="dark" /><div className="wrap">
      <div className="knowledge-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} /><span>Knowledge</span></div>
      <div className="knowledge-hero-copy"><span className="eyebrow">Help center, product docs, and team knowledge</span><h1 id="knowledge-title">Better docs for your customers.<br /><span>Better answers from your AI agents.</span></h1><p className="lede">Publish help articles, product guides, and API docs in one place. AI agents turn customer questions and product changes into draft updates for your team to review.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day cloud trial · No credit card required.</p><div className="knowledge-hero-points"><span><Server size={14} />Your own domain</span><span><Search size={14} />Search and AI answers</span><span><Code2 size={14} />Interactive API docs</span><span><ShieldCheck size={14} />Reviewed updates</span></div></div>
      <div className="knowledge-screenshot"><KnowledgeWorkspace /><p className="knowledge-demo-caption">A place for the answers your customers need.</p></div>
    </div></section>
    <nav className="knowledge-page-nav" aria-label="On this page"><div className="wrap"><strong>Knowledge</strong><a href="#knowledge-library">Reader experience</a><a href="#knowledge-publishing">Publishing & performance</a><a href="#knowledge-api">Developer docs</a><a href="#knowledge-sources">Agent knowledge</a><a href="#knowledge-quill">Keep docs current</a></div></nav>
    <section id="knowledge-library"><div className="wrap"><SectionHead eyebrow="Help customers help themselves" title="Make the answer easy to find. And the next step easy to follow." lede="Give customers a help center they can browse, search, or ask a question. Bring clear instructions and source-linked AI answers into the same reading experience." /><div className="knowledge-reader-image"><KnowledgeReader /><p className="knowledge-demo-caption">From the first question to a clear next step.</p></div><div className="knowledge-features">{FEATURES.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div><p className="knowledge-supporting-note">AI article translation is included in the Growth plan.</p></div></section>
    <section id="knowledge-publishing" className="knowledge-publishing-section section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><SectionHead eyebrow="Your help center. Your website." title="Keep your docs where customers expect them." lede="Use a Helpin address, connect your domain, or publish under a path such as /docs on your existing website through a reverse proxy." /><PublishingDemoV2 /><div className="knowledge-delivery-details"><article><span className="knowledge-detail-icon"><Server size={21}/></span><div><h3>Deliver the article, not just an empty page.</h3><p>Article content arrives in the first HTML response. Caching and compression support efficient delivery.</p><div className="knowledge-tech-points"><span>Server-rendered HTML</span><span>HTML caching</span><span>Response compression</span></div></div></article><article><span className="knowledge-detail-icon"><Search size={21}/></span><div><h3>Help search engines find your guides.</h3><p>Use metadata, canonical URLs, sitemaps, language links, and redirects to keep your published content discoverable.</p><div className="knowledge-tech-points"><span>Canonical URLs</span><span>Sitemaps & redirects</span><span>Social previews</span></div></div></article></div><p className="knowledge-proxy-platforms">Setup guidance for <span>Cloudflare Workers</span><span>AWS CloudFront</span><span>Vercel</span></p></div></section>
    <section id="knowledge-api"><div className="wrap"><SectionHead eyebrow="For the people building with your product" title="Help developers go from reading to making a request." lede="Import an OpenAPI specification to publish an interactive reference alongside your guides. Show developers how to authenticate, pass parameters, and try your API." /><div className="knowledge-reader-image knowledge-api-image"><KnowledgeAPI /><p className="knowledge-demo-caption">The endpoint, the example, and the request—in one place.</p></div><div className="knowledge-api-points"><span><Braces size={15}/>Switch code examples</span><span><Code2 size={15}/>Import an OpenAPI URL or file</span><span><ShieldCheck size={15}/>Try requests from the reference</span></div></div></section>
    <section id="knowledge-sources" className="knowledge-sources-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="The knowledge behind the answer" title="Give your agents the guidance you want them to use." lede="Choose the public docs, website pages, and supported files each agent can reference. Select a whole space, a collection, or individual articles—and check when the sources are ready." /><div className="knowledge-source-choice"><h3>Public guides and internal docs have different jobs.</h3><p>Keep customer-facing guidance public and team playbooks internal. Publishing a document and selecting it as agent knowledge are separate settings.</p><p>Shared knowledge should be a choice—not an assumption.</p></div></div><KnowledgeScene variant="sources" /></div></section>
    <section id="knowledge-gaps" className="knowledge-gaps-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="Let customer questions guide the next update" title="Your next useful article may start in a support conversation." lede="Use unanswered questions and incomplete guidance to find what your docs are missing. Give your team a starting point: the customer’s question, the existing answer, and what needs to be clearer." /><Link className="knowledge-inline-link" href="/new/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><SupportKnowledge variant="knowledge" /></div></section>
    <section id="knowledge-quill" className="knowledge-quill-section"><div className="wrap"><div className="knowledge-split"><div><span className="knowledge-agent-label"><img src="/new/agents/quill.svg" width={32} height={32} alt="" />Meet the docs agent</span><SectionHead eyebrow="Keep the guidance connected to the product" title="The product changed. Help the docs catch up." lede="Ask the docs agent to review the relevant guides, inspect the available product evidence, and prepare an update. Give your team the proposed edit and the reason behind it—not another article to start from scratch." /><Link className="knowledge-inline-link" href="/new/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><div><KnowledgeScene variant="quill" /><p className="knowledge-demo-caption">The instructions reflect the product—not an outdated version of it.</p></div></div><div className="knowledge-features knowledge-authoring">{AUTHORING.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true"/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-faq"><div className="wrap knowledge-faq-grid"><SectionHead eyebrow="Before you publish" title="Get to know Helpin Knowledge." /><div className="knowledge-faqs"><FAQList items={FAQS} className="faq-items" /><Link className="knowledge-inline-link" href="/new/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="knowledge-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Make the next answer more useful</span><h2 id="knowledge-final-title">Help the next customer<br />find the answer.</h2><p className="lede">Keep your help center, product docs, and AI knowledge connected to the questions customers ask and the changes your team ships.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day cloud trial · No credit card required.</p></div></div></section>
  </main><PreviewFooter homepage /></>;
}
