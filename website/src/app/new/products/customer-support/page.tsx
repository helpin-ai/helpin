import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, ChevronRight, Inbox, MessageSquare, Clock3, Languages, Paperclip, Search, Sparkles, Users, Zap } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { SectionHead, SIGNUP_URL, GITHUB_URL } from '../../_components/ui';
import { SupportScene } from './support-scene';
import { SupportControls } from './support-controls';
import { SupportKnowledge } from './support-knowledge';
import { SupportInboxFeatures } from './support-inbox-features';
import { SupportInboxShowcase } from './support-inbox-showcase';
import { SupportWorkspace } from './support-workspace';
import { SupportHeroScene } from './support-hero-scene';
import './support.css';

export const metadata: Metadata = {
  title: 'Customer Support — Helpin',
  description: 'AI agents answer customers using your knowledge and customer history. Keep human handoffs and product work connected to the original conversation.',
  alternates: { canonical: '/new/products/customer-support' },
  robots: { index: false, follow: false },
};

const FEATURES = [
  { icon: MessageSquare, title: 'Internal notes & mentions', copy: 'Discuss the next step privately and mention a teammate inside the conversation.' },
  { icon: Zap, title: 'Saved replies', copy: 'Reuse answers with shortcuts and customer variables, then edit them before sending.' },
  { icon: Users, title: 'Team presence', copy: 'See who is viewing or typing in a conversation before you jump in with another reply.' },
  { icon: Search, title: 'Conversation search', copy: 'Find past questions and replies without opening every thread in your inbox.' },
  { icon: Languages, title: 'Message translation', copy: 'Read incoming messages in your language and translate outgoing replies for the customer.' },
  { icon: Clock3, title: 'Office hours & reply times', copy: 'Set working hours and expected reply times so customers know when your team is available.' },
  { icon: Paperclip, title: 'Files & attachments', copy: 'Share screenshots and files in the conversation so the details stay with the request.' },
  { icon: Sparkles, title: 'AI follow-ups', copy: 'Configure follow-ups for quiet conversations and let your team cancel a pending follow-up when needed.' },
];

const FAQS = [
  ['Can we use Helpin for both live chat and email?', 'Yes. Helpin brings chat and email conversations into the support inbox. Add the support widget to your product and configure support email for your workspace. Self-hosted email delivery requires the optional Postmark integration.'],
  ['Can we keep our existing support email address?', 'Yes. Set up forwarding from your existing address to the Helpin forwarding address for your shared or team inbox, then verify it with a test email. To reply from your own domain, configure and verify a sender address as well.'],
  ['How do routing and assignment work?', 'Routing rules can match message content or sender details and direct conversations to a team inbox. AI triage can suggest a destination; automatic moves can be enabled in your settings. Team inboxes support manual assignment, with round-robin assignment available on eligible plans.'],
  ['Can we organize the inbox around our own workflow?', 'Yes. Create conversation tags and combine filters such as owner, status, and tags into saved views. Keep views personal or share them with the team.'],
  ['Where do AI answers come from?', 'Choose the docs, website pages, and uploaded files available to your agents. Coverage gaps help your team identify missing knowledge and review suggested articles or updates before publishing.'],
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

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-operations">Inbox tools</a><a href="#support-knowledge">Knowledge</a><a href="#support-workflow">AI & teamwork</a><a href="#support-connected">Connected work</a><a href="#support-controls">Controls</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="Start with the customer" title="Every conversation. The context to answer." lede="Bring live chat and forwarded email into one inbox. Route conversations to the right team, organize them with tags and saved views, and keep customer history beside every reply." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-operations" className="support-operations">
        <div className="wrap">
          <SectionHead eyebrow="Built for the whole support day" title="Bring it in. Route it. Keep it organized." lede="Connect your support email, give every conversation a destination, and build the views your team works from. The everyday inbox tools are here, alongside your agents." />
          <SupportInboxFeatures />
          <h3 className="support-tools-heading">The details that make teamwork work.</h3>
          <div className="support-features support-operation-tools">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-knowledge" className="support-knowledge-section">
        <div className="wrap support-knowledge-grid">
          <SectionHead eyebrow="Knowledge that gets better" title="Better answers start with what your team knows." lede="Connect your docs, website, and files. Find questions your knowledge doesn’t cover, then review suggested articles and updates." />
          <SupportKnowledge />
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
        </div>
      </section>

      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="When the answer needs a fix, keep it moving." lede="Discuss the issue with Ask Agent, create a linked task, and bring in a coding agent to prepare a fix. Assign it to your team for review, with the customer conversation attached." /><Link className="btn-link" href="/new#ask-agent">Explore Ask Agent <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <div className="support-connected-preview"><SupportWorkspace variant="agent" /></div>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Your support. Your settings." title="Choose when agents act and when people step in." lede="Set how AI responds in your inbox, which tools an agent can use, and which actions need approval." />
          <SupportControls />
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="A few useful answers" title="Before you open the inbox." /><div className="support-faqs">{FAQS.map(([question, answer]) => <details key={question}><summary>{question}</summary><p>{answer}</p></details>)}<a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the Community guide <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Start with a conversation</span><h2 id="support-final-cta-title">Give customers an answer.<br />Give your team a way forward.</h2><p className="lede">Bring AI answers, human support, and the work behind each request into one connected workspace.</p><div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Self-host Helpin <ArrowRight size={16} aria-hidden="true" /></a></div><p className="support-reassurance">14-day cloud trial · No credit card required</p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
