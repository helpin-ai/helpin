import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, ChevronRight, Inbox } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { SectionHead, SIGNUP_URL, GITHUB_URL } from '../../_components/ui';
import { SupportScene } from './support-scene';
import { SupportControls } from './support-controls';
import { SupportInboxFeatures } from './support-inbox-features';
import { SupportInboxShowcase } from './support-inbox-showcase';
import { SupportWorkspace } from './support-workspace';
import { SupportHeroScene } from './support-hero-scene';
import './support.css';

export const metadata = previewMetadata("Support \u2014 Helpin", "/new/products/customer-support");


const FAQS = [
  [
    "Can we use Helpin for both live chat and email?",
    "Yes. Add the chat widget and connect your support email to bring both channels into one inbox.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/configuration.md"
  ],
  [
    "Can we keep our existing support email address?",
    "Yes. Forward incoming mail to Helpin and verify your sender address to reply from your own domain.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/configuration.md"
  ],
  [
    "How do routing and assignment work?",
    "Rules route conversations to the right team inbox. Your team can assign owners and choose how AI triage helps.",
    "/new/products/customer-support#support-operations"
  ],
  [
    "Can we organize the inbox around our own workflow?",
    "Yes. Combine tags, owners and statuses into saved views, then keep them personal or share them with your team.",
    "/new/products/customer-support#support-inbox"
  ],
  [
    "Where do AI answers come from?",
    "Choose the docs, website pages and files agents can use. Coverage gaps show which questions need better documentation.",
    "/new/products/knowledge"
  ],
  [
    "Can agents reply directly to customers?",
    "Yes. Choose AI-first replies, internal assistance, or turn automatic replies off.",
    "/new/products/customer-support#support-controls"
  ],
  [
    "What happens when a customer needs a person?",
    "The conversation goes to your team with the customer’s messages, AI replies and handoff notes attached.",
    "/new/products/customer-support#support-workflow"
  ],
  [
    "How does support connect to product work?",
    "Create a task from a conversation and keep the customer request attached, so the people doing the work know who needs a follow-up.",
    "/new/products/projects"
  ],
  [
    "Can we self-host Support?",
    "Yes. Support, docs and agents are in the free Community edition.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ]
] as const;

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={DEMO_URL}>Book a demo <ArrowRight size={16} aria-hidden="true" /></a></div>;
}

export default function CustomerSupportPage() {
  return <>
    <PreviewNav />
    <div className="support-page">
      <section className="hero support-hero">
        <div className="wrap">
          <nav className="support-breadcrumb" aria-label="Breadcrumb"><Link href="/new/product">Products</Link><ChevronRight size={12} aria-hidden="true" /><span aria-current="page">Customer Support</span></nav>
          <div className="support-hero-grid">
            <div className="hero-inner">
              <Availability category="AI support inbox" />
              <h1>Support that ends with a fix, <span>not a ticket number.</span></h1>
              <p className="lede">Helpin AI answers from your docs and customer history. When a request needs a person or a product change, the conversation goes with it — to a teammate, a task or a coding agent — so your team can follow up when it ships.</p>
              <Actions />
              <CtaNote trial support />
            </div>
            <div><SupportHeroScene /></div>
          </div>
        </div>
      </section>

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-operations">Inbox tools</a><a href="#support-knowledge">Knowledge</a><a href="#support-workflow">AI & teamwork</a><a href="#support-identity">Identity & tools</a><a href="#support-connected">Connected work</a><a href="#support-controls">Controls</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="The inbox" title="One inbox for chat and email, with customer history beside every reply." lede="Bring live chat and forwarded email into one inbox. Route conversations to the right team, organize them with tags and saved views, and keep customer history beside every reply." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-operations" className="support-operations">
        <div className="wrap">
          <SectionHead eyebrow="Inbox basics" title="Email forwarding, routing rules and saved views — the everyday tools, done." lede="Connect your support email, give every conversation a destination, and build the views your team works from. The everyday inbox tools are here, alongside your agents." />
          <SupportInboxFeatures />
          <p className="support-also-included"><strong>Also included:</strong> internal notes and @mentions, saved replies, team presence, search, translation, office hours, attachments, and AI follow-ups.</p>
        </div>
      </section>

      <section id="support-knowledge" className="support-knowledge-section"><div className="wrap">
        <SectionHead eyebrow="Knowledge that gets better" title="Answers come from your docs. Gaps become new articles." lede="Connect your docs, website and files. Find questions your knowledge doesn’t cover, then review suggested articles and updates." />
        <Link className="btn-link" href="/new/products/knowledge#knowledge-gaps">Explore knowledge coverage →</Link>
      </div></section>

      <section id="support-workflow" className="support-workflow-section">
        <div className="wrap">
          <SectionHead eyebrow="AI and your team, in the same conversation" title="AI answers. Your team steps in when needed." lede="Let agents handle the first reply. When a question needs a person or a product change, keep the context with the work." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Let agents take the first reply.</h3><p>Answer customers using product knowledge and customer history. Choose direct AI replies or keep your team involved with internal assistance and draft review.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Bring in your team without starting over.</h3><p>Pass the conversation to a teammate with the question, previous replies, and internal notes together. Customers can pick up where they left off.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Keep the request attached to the work.</h3><p>Turn the request into a task, keep the customer linked, and prepare a follow-up when the work is ready.</p></div><SupportScene variant="followup" /></article>
          </div>
        </div>
      </section>


      <section id="support-identity" className="support-identity-section"><div className="wrap support-identity-summary">
        <h3>Verified customers, connected tools.</h3><p>Sign customers in from your backend and let agents check your logs before they reply.</p>
        <Link className="btn-link" href="/new/developers#identity">Developer docs →</Link>
      </div></section>

      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="When the answer is a bug, hand it to a coding agent." lede="Discuss the issue with Ask Agent, create a linked task, and bring in a coding agent to prepare a fix. Assign it to your team for review, with the customer conversation attached." /><Link className="btn-link" href="/new#ask-agent">Explore Ask Agent <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <div className="support-connected-preview"><SupportWorkspace variant="agent" panelOnly /></div>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="Choose when agents act and when people step in." lede="Set how AI responds in your inbox, which tools an agent can use, and which actions need approval." />
          <SupportControls />
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions" title="Before you open the inbox." /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /><a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the Community guide <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Start with a conversation</span><h2 id="support-final-cta-title">Every customer question ends with an answer or a fix.</h2><p className="lede">Bring AI answers, human support, and the work behind each request into one connected workspace.</p><Actions /><CtaNote trial support /></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
