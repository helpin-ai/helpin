import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowRight, BookOpen, Check, ChevronRight, Inbox, Mail, MessageSquare, MessagesSquare, PanelRight, Tag, Users } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { SectionHead, SIGNUP_URL, GITHUB_URL } from '../../_components/ui';
import { SupportScene } from './support-scene';
import { SupportHeroScene } from './support-hero-scene';
import './support.css';

export const metadata: Metadata = {
  title: 'Customer Support — Helpin',
  description: 'Bring chat, email, customer history, and AI assistance into one shared inbox. Turn customer questions into answers and connected work.',
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
  ['Can our team review AI replies?', 'Your team can review and edit AI drafts before sending. Agents also have tool access and approval settings, so you can choose how they participate in your workflow.'],
  ['How does support connect to product work?', 'In the full workspace, you can create tasks from a conversation and link customer requests to projects. That gives the team doing the work the original customer context, and helps support see who needs a follow-up.'],
  ['Can we self-host Customer Support?', 'Helpin Community includes support, docs, and agents, and runs with Docker Compose on your infrastructure. Projects and CRM are outside the default Community scope. Check the Community guide for current availability and setup requirements.'],
];

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href="#support-workflow">See how it works <ArrowRight size={16} aria-hidden="true" /></a></div>;
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
              <span className="eyebrow"><Inbox size={15} aria-hidden="true" /> Customer support, connected</span>
              <h1>Better support starts with <span>the full picture.</span></h1>
              <p className="lede">Give your team and AI agents the context to answer well. Bring chat, email, customer history, and the work behind each request into one shared inbox.</p>
              <Actions />
              <p className="support-reassurance">14-day cloud trial <span>·</span> No credit card required</p>
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-workflow">AI & teamwork</a><a href="#support-connected">Connected work</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap support-intro-grid">
          <SectionHead eyebrow="Start with the customer" title="A familiar inbox. A better starting point." lede="The last conversation. The setup question. The feature they’re waiting for. Keep the details beside the thread, so nobody has to ask the customer to start over." />
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
          <SectionHead eyebrow="AI and your team, in the same conversation" title="Let agents help. Keep the human touch." lede="Get a useful draft, bring in the right teammate, and carry the context into the next step." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Draft a useful answer.</h3><p>Draft answers from product knowledge and customer history. Your team can review, edit, and send.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Bring in the right teammate.</h3><p>Keep the conversation, internal notes, and customer details together when someone needs to step in.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Keep the next step connected.</h3><p>Turn the request into a task, keep the customer linked, and prepare a follow-up when the work is ready.</p></div><SupportScene variant="followup" /></article>
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
          <div><SectionHead eyebrow="Beyond the inbox" title="Don’t lose the customer when the work moves on." lede="Some answers need a product change. Connect the conversation to a project so engineering knows why it matters—and support knows who is waiting." /><Link className="btn-link" href="/new/product#projects">Explore connected projects <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <div className="support-work-card">
            <div className="support-work-card-top"><span className="support-work-id">SSO / 142</span><span className="support-work-status"><span className="support-live-dot" /> In progress</span></div>
            <h3>Support Okta group-to-role mapping</h3><p>Give Northstar’s admins a clear path from their pilot to a team-wide rollout.</p>
            <div className="support-work-owner"><img src="/new/avatars/sam.webp" width={26} height={26} alt="" /><span>Sam Rivera</span><span>SSO Enterprise Readiness</span></div>
            <div className="support-work-request"><span className="support-step-number">ORIGINAL CUSTOMER REQUEST</span><p>“Our security team needs the setup steps before we invite everyone.”</p><span>Maya Chen · Northstar Labs</span></div>
            <div className="support-work-bottom"><Mail size={16} aria-hidden="true" /><span>The customer is still part of the story.</span></div>
          </div>
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="A few useful answers" title="Before you open the inbox." /><div className="support-faqs">{FAQS.map(([question, answer]) => <details key={question}><summary>{question}</summary><p>{answer}</p></details>)}<a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the Community guide <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta"><div className="wrap"><div className="final"><span className="eyebrow">Start with a conversation</span><h2>Make the next reply<br />a better one.</h2><p className="lede">Give your team and agents the customer context they need to help.</p><div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Self-host Helpin <ArrowRight size={16} aria-hidden="true" /></a></div><p className="support-reassurance">14-day cloud trial · No credit card required</p></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
