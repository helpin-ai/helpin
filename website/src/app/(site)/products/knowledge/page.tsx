import { WorkScene } from '../../_components/WorkScene';
import { HeroVortex } from '../../_components/HeroVortex';
import { DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, BookOpen, Code2, FolderOpen, History, Languages, ShieldCheck, Search, Server, Braces } from 'lucide-react';
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

export const metadata = createPageMetadata(PAGE_SEO.knowledge);
const FEATURES = [
  { Icon: FolderOpen, title: 'Customers find the right guide.', body: 'Collections help customers find the right article. A table of contents takes them straight to the step they need, without reading the whole guide.' },
  { Icon: Search, title: 'Customers ask in their own words.', body: 'Customers can ask a question and get an AI answer based on your articles. Citations take them to the source when they want more detail.' },
  { Icon: Languages, title: 'Customers read in their language.', body: 'Customers can browse your help center and read guides in their preferred language. Your team reviews and publishes each translation, so the instructions stay clear.' },
];
const AUTHORING = [
  { Icon: Code2, title: 'Check the change first.', body: 'Give Quill the released change and permitted product access so the guide follows what customers can actually use.' },
  { Icon: BookOpen, title: 'Show the steps clearly.', body: 'Browser tools can capture a screenshot or short recording for a document. Review the capture and access settings before sharing it.' },
  { Icon: History, title: 'Review once. Publish when ready.', body: 'See what changed and why. Keep the existing article live while you review the draft.' },
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
    "Can we import docs from Zendesk or Intercom?",
    "Zendesk and Intercom import is coming soon."
  ],
  [
    "Can we self-host Knowledge?",
    "Yes. Community includes every module by default, including docs and the help center. It is free under AGPL-3.0 with no plan limits. Community is currently a 0.2 beta.",
    "/self-hosting#whats-included",
    "See what’s included"
  ]
] as const;
export default function KnowledgePage() {
  return <><PreviewNav tone="dark" /><main className="knowledge-page">
    <section className="knowledge-hero motion-hero" aria-labelledby="knowledge-title"><HeroVortex variant="flow" tone="dark" /><div className="wrap">

      <div className="knowledge-hero-copy"><span className="eyebrow">Help center, product docs, and team knowledge</span><h1 id="knowledge-title">Help docs that keep up.<br /><span>An agent to do the upkeep.</span></h1><p className="lede">The Quill agent prepares guide updates from customer questions and product changes. Give people clearer instructions and your support agent better answers. Your team reviews and publishes.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day free trial · No card required</p><div className="knowledge-hero-points"><span><Server size={14} />Your own domain</span><span><Search size={14} />Search and AI answers</span><span><Code2 size={14} />Interactive API docs</span><span><ShieldCheck size={14} />Your team publishes</span></div></div>
      <div className="knowledge-screenshot"><KnowledgeWorkspace /><p className="knowledge-demo-caption">Collections, articles, and the API reference in one space.</p></div>
    </div></section>

    <section id="knowledge-library"><div className="wrap"><SectionHead eyebrow="Help center" title="A help center where customers find answers." lede="Let customers browse a guide or ask a question. AI answers link to the articles behind them. When there is not enough evidence for an answer, the relevant guides stay easy to find." /><div className="knowledge-reader-image"><KnowledgeReader /><p className="knowledge-demo-caption">A table of contents, search, and AI answers on every article.</p></div><div className="knowledge-features">{FEATURES.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true" /><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-publishing" className="knowledge-publishing-section section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><SectionHead eyebrow="Your help center. Your website." title="Make the help center feel like part of your product." lede="Use a Helpin address, connect your domain, or publish under a path such as /docs on your existing website through a reverse proxy." /><PublishingDemoV2 /><div className="knowledge-delivery-details"><article><span className="knowledge-detail-icon"><Server size={21}/></span><div><h3>Open the guide. Start reading.</h3><p>Readers receive the article with the page. Server rendering, caching, and compression help it load quickly.</p><div className="knowledge-tech-points"><span>Server-rendered HTML</span><span>HTML caching</span><span>Response compression</span></div></div></article><article><span className="knowledge-detail-icon"><Search size={21}/></span><div><h3>Help search engines find your guides.</h3><p>Use metadata, canonical URLs, sitemaps, language links, and redirects to keep your published content discoverable.</p><div className="knowledge-tech-points"><span>Canonical URLs</span><span>Sitemaps & redirects</span><span>Social previews</span></div></div></article></div><p className="knowledge-proxy-platforms">Setup guidance for <span>Cloudflare Workers</span><span>AWS CloudFront</span><span>Vercel</span><span>nginx</span></p></div></section>
    <section id="knowledge-api"><div className="wrap"><SectionHead eyebrow="For the people building with your product" title="Give developers an example they can try." lede="Import an OpenAPI specification to publish an interactive reference alongside your guides. Show developers how to authenticate, pass parameters, and try your API." /><div className="knowledge-reader-image knowledge-api-image"><KnowledgeAPI /><p className="knowledge-demo-caption">Read the example. Try the request.</p></div><div className="knowledge-api-points"><span><Braces size={15}/>Switch code examples</span><span><Code2 size={15}/>Import an OpenAPI URL or file</span><span><ShieldCheck size={15}/>Try requests from the reference</span></div></div></section>
    <section id="knowledge-sources" className="knowledge-sources-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="The knowledge behind the answer" title="Choose what your agents learn from." lede="Give Echo the help articles it needs and Quill the guidance it should maintain. Select docs, website pages, or supported files. Published changes automatically refresh the searchable knowledge." /><div className="knowledge-source-choice"><h3>Keep team instructions separate from public guides.</h3><p>Keep customer-facing guidance public and team playbooks internal. Publishing a document and selecting it as agent knowledge are separate settings.</p></div></div><KnowledgeScene variant="sources" /></div></section>
    <section id="knowledge-gaps" className="knowledge-gaps-section"><div className="wrap knowledge-split"><div><SectionHead eyebrow="Let customer questions guide the next update" title="Turn repeated questions into a helpful guide." lede="See the questions behind a knowledge gap, including support conversations and unsuccessful searches. Ask the Quill agent to improve the existing guide or draft the missing one." /><Link prefetch={false} className="knowledge-inline-link" href="/products/customer-support">See how support connects<ArrowRight size={15} /></Link></div><SupportKnowledge variant="knowledge" /></div></section>
    <section id="knowledge-quill" className="knowledge-quill-section"><div className="wrap"><div className="knowledge-split"><div><span className="knowledge-agent-label"><img src="/new/agents/quill.svg" width={32} height={32} alt="" />Meet Quill, your docs agent</span><SectionHead eyebrow="AI agents keep your docs current" title="Your product changed. Your guide should too." lede="A new release can leave guides behind. Quill prepares updates on a schedule, after a supported event, or when you ask. Add fresh steps, screenshots, and browser recordings to the draft." /><Link prefetch={false} className="knowledge-inline-link" href="/products/ai-agents">Meet the agents<ArrowRight size={15} /></Link></div><div><WorkScene variant="docs" /><p className="knowledge-demo-caption">Checked against the released change before you review it.</p></div></div><div className="knowledge-features knowledge-authoring">{AUTHORING.map(({Icon,title,body})=><article key={title}><Icon size={20} aria-hidden="true"/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="knowledge-faq"><div className="wrap knowledge-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s Knowledge tool" /><div className="knowledge-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="knowledge-inline-link" href="/self-hosting">Explore self-hosting<ArrowRight size={15} /></Link></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="knowledge-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><h2 id="knowledge-final-title">Give your guides<br />a helping hand.</h2><p className="lede">Keep your docs current with the Quill agent, so your customers and Echo have reliable guidance to work with.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="knowledge-supporting-note">14-day free trial · No card required</p><p className="knowledge-supporting-note">Open source · Self-host free, or let us run it</p></div></div></section>
  </main><PreviewFooter homepage /></>;
}
