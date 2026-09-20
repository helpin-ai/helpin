import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, BookOpen, Check, ChevronRight, Inbox, Mail, MessageSquare, MessagesSquare, PanelRight, ShieldCheck, SlidersHorizontal, Tag, Users, Wrench } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { SectionHead, SIGNUP_URL, GITHUB_URL } from '../../_components/ui';
import { SupportScene } from './support-scene';
import { SupportHeroScene } from './support-hero-scene';
import './support.css';

export const metadata: Metadata = {
  title: 'Customer Support — Helpin',
  description: 'AI agents answer customers using your knowledge and customer history. Keep human handoffs and product work connected to the original conversation.',
  alternates: { canonical: '/new/products/customer-support' },
  robots: { index: false, follow: false },
};

const FEATURES = [
  { icon: MessagesSquare, title: 'Chat and email, together', copy: 'Keep customer conversations in a shared inbox, with the history ready when your team replies.' },
  { icon: Users, title: 'Clear ownership for every reply', copy: 'Assign conversations to teammates and organize work with team inboxes.' },
  { icon: MessageSquare, title: 'Internal notes. Shared context.', copy: 'Leave internal notes so the next teammate knows what has already been tried.' },
  { icon: Tag, title: 'Tags that keep work organized', copy: 'Use tags and conversation states to keep requests organized and follow-ups visible.' },
  { icon: BookOpen, title: 'Knowledge within reach', copy: 'Use help articles and saved replies to give useful answers without starting from scratch.' },
  { icon: PanelRight, title: 'Customer context beside the thread', copy: 'See who you are helping, their company, and linked work while you respond.' },
];

const FAQS = [
  ['Can we use Helpin for both live chat and email?', 'Yes. Helpin brings chat and email conversations into the support inbox. Add the support widget to your product and configure support email for your workspace. Self-hosted email delivery requires the optional Postmark integration.'],
  ['Can agents reply directly to customers?', 'Yes. Enable AI-first replies for your support inbox to let the configured agent respond to customers. You can also use internal AI assistance or turn automatic replies off. Your team can review and edit drafts before sending.'],
  ['What happens when a customer needs a person?', 'Helpin can hand the conversation over to your team. The customer’s messages, AI replies, and handoff notes stay in the thread so your team can continue with the context in view.'],
  ['How does support connect to product work?', 'In the full workspace, you can create tasks from a conversation and link customer requests to projects. That gives the team doing the work the original customer context, and helps support see who needs a follow-up.'],
  ['Can we self-host Customer Support?', 'Helpin Community includes support, docs, and agents, and runs with Docker Compose on your infrastructure. Projects and CRM are outside the default Community scope. Check the Community guide for current availability and setup requirements.'],
];

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href="#support-workflow">See it in action <ArrowRight size={16} aria-hidden="true" /></a></div>;
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
              <span className="eyebrow"><Inbox size={15} aria-hidden="true" /> AI customer support, connected to your product</span>
              <h1>Answer the question. <span>Move the issue forward.</span></h1>
              <p className="lede">Helpin agents answer customers using your knowledge and customer history. When a request needs your team, keep the conversation connected to the people and product work that can resolve it.</p>
              <Actions />
              <p className="support-reassurance">14-day cloud trial <span>·</span> No credit card required</p>
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-workflow">AI & teamwork</a><a href="#support-connected">Connected work</a><a href="#support-controls">Controls</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap support-intro-grid">
          <SectionHead eyebrow="Start with the customer" title="Every conversation. The context to answer." lede="Bring chat, email, customer history, and linked work into one shared inbox. Your team and agents can see what came before and what still needs attention." />
          <div className="support-context-card">
            <div className="support-context-person"><img src="/new/avatars/maya.webp" alt="" width={48} height={48} /><div><strong>Maya Chen</strong><span>Northstar Labs · Customer</span></div><span className="support-context-tag">SSO rollout</span></div>
            <div className="support-context-row"><MessageSquare size={18} aria-hidden="true" /><div><span>THE CONVERSATION</span><p>“Can we pilot Okta with our admins first?”</p></div></div>
            <div className="support-context-row"><BookOpen size={18} aria-hidden="true" /><div><span>THE KNOWLEDGE</span><p>Okta setup guide, ready to share</p></div></div>
            <div className="support-context-row"><PanelRight size={18} aria-hidden="true" /><div><span>THE LINKED WORK</span><p>SSO Enterprise Readiness <small>In progress</small></p></div></div>
            <div className="support-context-foot"><span className="support-live-dot" /> One customer record. Right beside the reply.</div>
          </div>
        </div>
        <div className="wrap">
          <figure className="support-product-shot">
            <div className="support-shot-heading"><span><span className="support-live-dot" /> One inbox. The whole customer story.</span><span className="support-demo-label">EXAMPLE WORKSPACE</span></div>
            <a href="/new/support/inbox-1672-v2.webp" target="_blank" rel="noopener noreferrer" aria-label="View the full-size support inbox illustration (opens in a new tab)">
              <img src="/new/support/inbox-1672-v2.webp" srcSet="/new/support/inbox-960-v2.webp 960w, /new/support/inbox-1672-v2.webp 1672w" sizes="(max-width: 1240px) calc(100vw - 48px), 1192px" width={1672} height={941} alt="Illustrative OrbitDesk inbox: Maya Chen at Northstar Labs asks about an Okta pilot. Sam’s internal note, an AI reply draft, customer details, and the linked SSO project appear together." loading="lazy" decoding="async" />
            </a>
            <figcaption><span><MessageSquare size={15} aria-hidden="true" /> The question</span><ChevronRight size={13} aria-hidden="true" /><span><PanelRight size={15} aria-hidden="true" /> The context</span><ChevronRight size={13} aria-hidden="true" /><span><Check size={15} aria-hidden="true" /> The next step</span></figcaption>
          </figure>
        </div>
      </section>

      <section id="support-workflow" className="support-workflow-section">
        <div className="wrap">
          <SectionHead eyebrow="AI and your team, in the same conversation" title="AI answers. Your team steps in when needed." lede="Let agents handle the first reply. When a question needs a person or a product change, keep the context with the work." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Let agents take the first reply.</h3><p>Answer customers using product knowledge and customer history. Choose direct AI replies or keep your team involved with internal assistance and draft review.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Bring in your team without starting over.</h3><p>Pass the conversation to a teammate with the question, previous replies, and internal notes together. Customers can pick up where they left off.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Keep the request attached to the work.</h3><p>Turn the request into a task, keep the customer linked, and prepare a follow-up when the work is ready.</p></div><SupportScene variant="followup" /></article>
          </div>
          <p className="support-workflow-note">Illustrative workflows. Project connections are available in the full workspace.</p>
        </div>
      </section>

      <section className="support-essentials">
        <div className="wrap">
          <SectionHead eyebrow="Made for the day-to-day" title="Everything a good reply needs." />
          <div className="support-features">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="When the answer needs a fix, keep it moving." lede="Turn bugs and feature requests into linked tasks and projects. Engineering gets the original conversation. Support can see the work and prepare an update for the customers waiting on it." /><Link className="btn-link" href="/new/product#projects">Explore connected projects <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <div className="support-work-card">
            <div className="support-work-card-top"><span className="support-work-id">SSO / 142</span><span className="support-work-status"><span className="support-live-dot" /> In progress</span></div>
            <h3>Investigate incorrect Okta group roles</h3><p>Check why Northstar’s admin group receives the wrong role before the wider rollout.</p>
            <div className="support-work-owner"><img src="/new/avatars/sam.webp" width={26} height={26} alt="" /><span>Sam Rivera</span><span>SSO Enterprise Readiness</span></div>
            <div className="support-work-request"><span className="support-step-number">ORIGINAL CUSTOMER REQUEST</span><p>“The group mapping gives our admins the wrong role.”</p><span>Maya Chen · Northstar Labs</span></div>
            <div className="support-work-bottom"><Mail size={16} aria-hidden="true" /><span>The customer is still part of the story.</span></div>
          </div>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Your support. Your settings." title="Choose when agents act and when people step in." lede="Set how AI responds in your inbox, which tools an agent can use, and which actions need approval." />
          <div className="support-features">
            <article><SlidersHorizontal size={23} strokeWidth={1.5} aria-hidden="true" /><h3>Choose how AI responds.</h3><p>Use AI-first replies, keep assistance internal, or turn automatic replies off for your inbox.</p></article>
            <article><Wrench size={23} strokeWidth={1.5} aria-hidden="true" /><h3>Give agents the right tools.</h3><p>Select the tools each agent can use for its work, including tools from connected MCP servers.</p></article>
            <article><ShieldCheck size={23} strokeWidth={1.5} aria-hidden="true" /><h3>Review actions that need you.</h3><p>Configure agent approval settings so your team can review requested tool actions before they run.</p></article>
          </div>
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="A few useful answers" title="Before you open the inbox." /><div className="support-faqs">{FAQS.map(([question, answer]) => <details key={question}><summary>{question}</summary><p>{answer}</p></details>)}<a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the Community guide <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta"><div className="wrap"><div className="final"><span className="eyebrow">Start with a conversation</span><h2>Give customers an answer.<br />Give your team a way forward.</h2><p className="lede">Bring AI answers, human support, and the work behind each request into one connected workspace.</p><div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Self-host Helpin <ArrowRight size={16} aria-hidden="true" /></a></div><p className="support-reassurance">14-day cloud trial · No credit card required</p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
