import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, ChevronRight, Inbox, MessageSquare, Clock3, Languages, Paperclip, Search, ShieldCheck, Plug, Sparkles, Users, Zap } from 'lucide-react';
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
import { SupportKnowledge } from './support-knowledge';
import { SupportIdentity } from './support-identity';
import './support.css';

export const metadata = previewMetadata("Support \u2014 Helpin", "/new/products/customer-support");


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
    "Yes. Support is part of the open-source product and can run on your own infrastructure.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ],
  ["How is customer identity verified?", "Your server signs the customer’s identity, and Helpin checks the signature before marking them as verified.", "/new/developers#identity"],
  ["Does a verified identity automatically grant access to connected tools?", "No. Verification confirms who is asking; connected tools still enforce access to customer data, and agents use only the tools you allow.", "/new/developers#identity"]
] as const;

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={DEMO_URL}>Book a demo <ArrowRight size={16} aria-hidden="true" /></a></div>;
}

export default function CustomerSupportPage() {
  return <>
    <PreviewNav />
    <div className="support-page">
      <section className="hero support-hero motion-hero"><HeroVortex variant="flow" tone="dark" />
        <div className="wrap">
          <nav className="support-breadcrumb" aria-label="Breadcrumb"><Link href="/new/product">Products</Link><ChevronRight size={12} aria-hidden="true" /><span aria-current="page">Customer Support</span></nav>
          <div className="support-hero-grid">
            <div className="hero-inner">
              <Availability category="AI support inbox" />
              <h1><span className="support-headline-opening">Support that ends</span> with a fix, <span>not a ticket number.</span></h1>
              <p className="lede">Helpin AI answers from your docs, customer history and your own logs. When a request needs a fix, it goes to a teammate or a coding agent with the conversation attached — and the customer hears back when it ships.</p>
              <Actions />
              <CtaNote trial support />
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-operations">Inbox tools</a><a href="#support-knowledge">Knowledge</a><a href="#support-workflow">AI & teamwork</a><a href="#support-identity">Identity & tools</a><a href="#support-connected">Connected work</a><a href="#support-controls">Controls</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="The inbox" title="One inbox, from first question to follow-up." lede="Bring chat and email together. Helpin AI checks your docs and connected tools, then hands your team the findings, customer history and linked work when a person needs to step in." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-operations" className="support-operations section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Inbox basics" title="Email forwarding, routing rules and saved views — the everyday tools, done." lede="Connect your support email, give every conversation a destination, and build the views your team works from. The everyday inbox tools are here, alongside your agents." />
          <SupportInboxFeatures />
          <h3 className="support-tools-heading">Everyday tools for your support team.</h3>
          <div className="support-features support-operation-tools">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-knowledge" className="support-knowledge-section">
        <div className="wrap support-knowledge-grid">
          <SectionHead eyebrow="Knowledge that gets better" title="Answers come from your docs. Gaps become new articles." lede="Connect your docs, website and files. Find questions your knowledge doesn’t cover, then review suggested articles and updates." />
          <SupportKnowledge />
        </div>
      </section>

      <section id="support-workflow" className="support-workflow-section">
        <div className="wrap">
          <SectionHead eyebrow="AI and your team, in the same conversation" title="AI answers. Your team steps in when needed." lede="Answer from your docs, investigate with connected tools, and bring in the right teammate when needed. Keep the customer informed as the work moves forward." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Let agents take the first reply.</h3><p>Helpin AI answers from your docs and customer history, and checks connected tools for deeper questions. Choose direct replies or drafts your team reviews.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Bring in your team without starting over.</h3><p>Give the right teammate the conversation, log findings and customer history together. The investigation is already there, so nobody starts from scratch.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Follow up when the fix is live.</h3><p>Link the request to a task. Once the fix is reviewed, tested and released, Helpin AI sends the customer an update with your team’s approval.</p></div><SupportScene variant="followup" /></article>
          </div>
        </div>
      </section>


      <section id="support-identity" className="support-identity-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap support-identity-grid">
          <div className="support-identity-copy">
            <SectionHead eyebrow="Customer identity and tool access" title="Verified customers, connected tools." lede="Verify signed-in customers and let agents check connected logs and account tools before they reply." />
            <div className="support-identity-points">
              <div><ShieldCheck size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Verify who’s asking.</h3><p>Your server signs the customer’s identity. Helpin checks that signature before showing the customer as verified.</p></div></div>
              <div><Plug size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Bring your logs into the conversation.</h3><p>Connect logs and account tools, then choose which ones each agent can use. Your connected tools control which customer data they return.</p></div></div>
              <div><Sparkles size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Turn findings into a useful next step.</h3><p>Ask Agent investigates the issue, prepares a reply, and carries out permitted work. Choose which actions need your team’s approval.</p></div></div>
            </div>
            <div className="support-identity-links"><a href={`${GITHUB_URL}/blob/develop/docs/community/widget-identity.md`} target="_blank" rel="noopener noreferrer">Set up identity verification <ArrowRight size={13} aria-hidden="true" /></a><a href={`${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md`} target="_blank" rel="noopener noreferrer">Connect your tools <ArrowRight size={13} aria-hidden="true" /></a></div>
          </div>
          <SupportIdentity />
        </div>
      </section>

      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="When the answer is a bug, hand it to a coding agent." lede="Discuss the issue with Ask Agent, create a linked task, and bring in a coding agent to prepare a fix. Assign it to your team for review, with the customer conversation attached." /><Link className="btn-link" href="/new#ask-agent">Explore Ask Agent <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <div className="support-connected-preview"><SupportWorkspace variant="agent" /></div>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Approvals built in" title="Choose when agents act and when people step in." lede="Set how AI responds in your inbox, which tools an agent can use, and which actions need approval." />
          <SupportControls />
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions" title="Before you open the inbox." /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /><a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Read the self-hosting guide <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Start with a conversation</span><h2 id="support-final-cta-title">Every customer question ends with an answer or a fix.</h2><p className="lede">Bring AI answers, human support, and the work behind each request into one connected workspace.</p><Actions /><CtaNote trial support /></div></div></section>
    </div>
    <PreviewFooter />
  </>;
}
