import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, DEMO_URL, FAQList } from '../../_components/ui';
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
  { icon: MessageSquare, title: 'Internal notes and mentions', copy: 'Discuss the next step privately and bring teammates into the conversation.' },
  { icon: Zap, title: 'Saved replies', copy: 'Reuse common answers, then personalize them before sending.' },
  { icon: Users, title: 'Team presence', copy: 'See who’s viewing or typing before you reply.' },
  { icon: Search, title: 'Conversation search', copy: 'Find an earlier question without opening every thread.' },
  { icon: Languages, title: 'Message translation', copy: 'Read incoming messages and prepare replies across languages.' },
  { icon: Clock3, title: 'Office hours and reply times', copy: 'Let customers know when to expect your team.' },
  { icon: Paperclip, title: 'Files and attachments', copy: 'Keep screenshots and supporting files with the request.' },
  { icon: Sparkles, title: 'AI follow-ups', copy: 'Schedule conversation follow-ups and cancel pending messages when needed.' },
];

const FAQS = [
  [
    "Can our team handle chat and email together?",
    "Yes. Add the chat widget and forward support email into Helpin’s shared inbox.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/configuration.md"
  ],
  [
    "Do we need a new support email address?",
    "No. Keep your existing address and verify it for replies through Helpin.",
    "https://github.com/helpin-ai/helpin/blob/develop/docs/community/configuration.md"
  ],
  [
    "How are conversations assigned?",
    "Route requests to team inboxes and assign owners. Round-robin assignment and AI routing are available on eligible Cloud plans.",
    "#support-operations"
  ],
  [
    "Can we create views for our own support process?",
    "Yes. Filter conversations using tags, owners, and statuses, then save a personal or shared view.",
    "#support-inbox"
  ],
  [
    "What information can agents use to answer?",
    "Agents use the knowledge sources you select, including supported docs, website pages, and files. Relevant customer history and permitted connected tools can provide additional context.",
    "/new/products/knowledge"
  ],
  [
    "Do agents have to send replies automatically?",
    "No. Your team can use AI for internal assistance instead, or disable automatic responses.",
    "#support-controls"
  ],
  [
    "What does a teammate receive during a handoff?",
    "The customer’s conversation, the agent’s responses, and the handoff notes—not just a new assignment.",
    "#support-workflow"
  ],
  [
    "Can a support request become project work?",
    "Yes. Create or link a task and retain the original request, so your team can plan the work without losing who needs it.",
    "/new/products/projects"
  ],
  [
    "Is self-hosting available for Support?",
    "Yes. Support is included in Helpin’s open-source product. Your team operates the installation and covers hosting and provider costs; Enterprise features are licensed separately.",
    "/new/self-hosting#whats-included",
    "See what’s included"
  ],
  [
    "How does Helpin check a customer’s identity?",
    "Your backend signs an identity proof. Helpin validates it before marking the customer as verified in the widget.",
    "/new/developers#identity"
  ],
  [
    "Does verification give an agent access to all customer data?",
    "No. Identity verification does not grant tool permissions. You select the tools an agent may use, and your connected systems must enforce access to customer data.",
    "/new/developers#identity"
  ]
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
              <Availability category="Customer support for SaaS teams" />
              <h1>AI agents that know the history.<br /><span>Support that follows through.</span></h1>
              <p className="lede">Bring chat, email, and customer history into one inbox. AI agents use your product knowledge and connected tools to answer questions, investigate issues, and move requests forward—with your team in control.</p>
              <Actions />
              <p className="cta-note">14-day cloud trial · No credit card required · Self-hosting available.</p>
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>

      <nav className="support-page-nav" aria-label="On this page"><div className="wrap"><span>Customer Support</span><a href="#support-inbox">Shared inbox</a><a href="#support-operations">Inbox tools</a><a href="#support-knowledge">Knowledge</a><a href="#support-workflow">AI & teamwork</a><a href="#support-identity">Identity & tools</a><a href="#support-connected">Connected work</a><a href="#support-controls">Controls</a><a href="#support-faq">FAQs</a></div></nav>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="The shared inbox" title="One place to reply. The context to get it right." lede="Work through chat and email with the customer’s details, past conversations, and linked tasks in view. Your team and AI agents can pick up the request without piecing the story back together." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-operations" className="support-operations section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Everyday support" title="A shared inbox your team can work from." lede="Connect your support address, organize incoming requests, and give each conversation an owner." />
          <SupportInboxFeatures />
          <h3 className="support-tools-heading">The details that keep support moving.</h3>
          <div className="support-features support-operation-tools">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-knowledge" className="support-knowledge-section">
        <div className="wrap support-knowledge-grid">
          <SectionHead eyebrow="Better knowledge. Better answers." title="Turn unanswered questions into useful docs." lede="Choose the docs, website pages, and files your agents can use. When questions reveal missing guidance, turn the conversation into a draft article or update your team can review." />
          <SupportKnowledge />
        </div>
      </section>

      <section id="support-workflow" className="support-workflow-section">
        <div className="wrap">
          <SectionHead eyebrow="AI and your team" title="A handoff, not a restart." lede="Let agents handle questions using the available history and knowledge. When a person needs to take over, keep the conversation and investigation together—then carry the outcome back to the customer." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Start with what you already know.</h3><p>Use product knowledge, customer history, and selected tools to investigate the request. Choose direct AI replies or assistance your team reviews.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Pass on the findings, not just the ticket.</h3><p>Give the next teammate the customer’s messages, what’s been tried, and what the agent found.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Bring the outcome back to the customer.</h3><p>Keep the request linked to the work. Configure a follow-up after release, with the approvals your team requires.</p></div><SupportScene variant="followup" /></article>
          </div>
        </div>
      </section>


      <section id="support-identity" className="support-identity-section section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap support-identity-grid">
          <div className="support-identity-copy">
            <SectionHead eyebrow="Identity and access" title="Know who’s asking. Control what AI can access." lede="Verify signed-in customers and give agents selected tools to investigate their requests. Confirming identity and granting access remain separate controls." />
            <div className="support-identity-points">
              <div><ShieldCheck size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Verify the customer behind the message.</h3><p>Have your server sign the customer’s identity. Helpin checks that proof when the customer is identified in the widget.</p></div></div>
              <div><Plug size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Check the systems behind the issue.</h3><p>Connect selected logs and account tools. Your tool server determines which customer data each request may return.</p></div></div>
              <div><Sparkles size={21} strokeWidth={1.5} aria-hidden="true" /><div><h3>Turn the findings into action.</h3><p>Let Ask Agent investigate and prepare the next step using permitted tools. Set which actions require approval.</p></div></div>
            </div>
            <div className="support-identity-links"><a href={`${GITHUB_URL}/blob/develop/docs/community/widget-identity.md`} target="_blank" rel="noopener noreferrer">Configure customer identity <ArrowRight size={13} aria-hidden="true" /></a><a href={`${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md`} target="_blank" rel="noopener noreferrer">Connect external tools <ArrowRight size={13} aria-hidden="true" /></a></div>
          </div>
          <SupportIdentity />
        </div>
      </section>

      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="When a reply isn’t enough, move the work forward." lede="Create or link a task with the customer’s request attached. For a bug, bring in a coding agent to prepare a change for review. Keep support connected to the work—and to the person waiting for it." /><Link className="btn-link" href="/new#ask-agent">Explore Ask Agent <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <figure className="support-connected-preview"><h3 className="support-demo-heading">From reported issue to proposed change</h3><SupportWorkspace variant="agent" /><figcaption className="support-demo-caption">The work moves forward. The customer’s need stays attached.</figcaption></figure>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Your team sets the rules" title="Choose what AI handles. Decide where people step in." lede="Set how agents respond, which tools they can use, and when they need approval." />
          <SupportControls />
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions before you start" title="Get to know Helpin Support." /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /><a className="btn-link" href={`${GITHUB_URL}/blob/develop/community/README.md`} target="_blank" rel="noopener noreferrer">Explore self-hosting <ArrowRight size={16} aria-hidden="true" /></a></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">From question to next step</span><h2 id="support-final-cta-title">Help customers move forward.<br />Not start over.</h2><p className="lede">Bring your team, customer history, and AI agents into one workspace. Answer the question, connect the work, and keep the customer informed.</p><Actions /><p className="cta-note">14-day cloud trial · No credit card required · Self-hosting available.</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
