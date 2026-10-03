import { WorkScene } from '../../_components/WorkScene';
import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, MessageSquare, Clock3, Mail, Paperclip, Search, Sparkles, Users, Zap } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { SectionHead, SIGNUP_URL } from '../../_components/ui';
import { SupportScene } from './support-scene';
import { SupportControls } from './support-controls';
import { SupportInboxFeatures } from './support-inbox-features';
import { SupportInboxShowcase } from './support-inbox-showcase';
import { SupportWorkspace } from './support-workspace';
import { SupportHeroScene } from './support-hero-scene';
import './support.css';
import { DOCS } from '../../_components/docsLinks';

export const metadata = createPageMetadata(PAGE_SEO.customerSupport);


const FEATURES = [
  { icon: MessageSquare, title: 'Internal notes and mentions', copy: 'Discuss the next step privately and bring teammates into the conversation.' },
  { icon: Zap, title: 'Saved replies', copy: 'Reuse common answers, then personalize them before sending.' },
  { icon: Users, title: 'Team presence', copy: 'See who’s viewing or typing before you reply.' },
  { icon: Search, title: 'Conversation search', copy: 'Find an earlier question without opening every thread.' },
  { icon: Mail, title: 'Email forwarding', copy: 'Keep your support address. Forward incoming email to Helpin and reply from your verified address.' },
  { icon: Clock3, title: 'Office hours and reply times', copy: 'Let customers know when to expect your team.' },
  { icon: Paperclip, title: 'Files and attachments', copy: 'Keep screenshots and supporting files with the request.' },
  { icon: Sparkles, title: 'AI follow-ups', copy: 'The Echo agent can check whether the answer helped when a customer goes quiet. Cancel a pending follow-up whenever you need to.' },
];

const FAQS = [
  [
    "Can we reply to customers in another language?",
    "Yes. Turn on Live Translate for a conversation. Helpin AI translates new customer messages into your reading language and sends your replies in theirs. You can choose the customer’s language, pause translation and view the original text.",
    "#support-live-translate",
    "See Live Translate"
  ],
  [
    "Can our team handle chat and email together?",
    "Yes. Add the chat widget and forward support email into Helpin’s shared inbox.",
    DOCS.emailForwarding
  ],
  [
    "Do we need a new support email address?",
    "No. Keep your existing address and verify it for replies through Helpin.",
    DOCS.senderAddresses
  ],
  [
    "How are conversations assigned?",
    "Route requests to team inboxes and assign owners. Use round-robin assignment to share the load, or AI routing to match a conversation to the right team.",
    "#support-operations"
  ],
  [
    "Can we create views for our own support process?",
    "Yes. Filter conversations using tags, owners, and statuses, then save a personal or shared view.",
    "#support-inbox"
  ],
  [
    "What information can agents use to answer?",
    "The Echo agent uses the docs, website pages, and files you select. It can also use relevant conversations and permitted connected tools to check account facts. Your systems control which customer data those tools return.",
    "/products/knowledge"
  ],
  [
    "Do agents have to send replies automatically?",
    "No. Your team can use AI for internal assistance instead, or disable automatic responses.",
    "#support-controls"
  ],
  [
    "What does a teammate receive during a handoff?",
    "The customer’s messages, earlier replies, and the agent’s handoff notes stay in the conversation.",
    "#support-workflow"
  ],
  [
    "Can a support request become project work?",
    "Yes. Ask Agent can create a task or link an existing one, with the customer’s report attached. You can then bring in planning and coding agents to work on it.",
    "/products/projects"
  ],
  [
    "Can we import from Zendesk or Intercom?",
    "Import from Zendesk and Intercom is coming soon. Until then, forward your support email to Helpin and new conversations start there.",
    "/compare",
    "See how Helpin compares"
  ],
  [
    "Is self-hosting available for Support?",
    "Yes. Support is part of the open-source Community edition (0.2 beta), with every support feature and no plan limits. Your team operates the installation and covers hosting and provider costs.",
    "/self-hosting#whats-included",
    "See what’s included"
  ],
  [
    "How does Helpin check a customer’s identity?",
    "Your backend signs an identity proof. Helpin validates it before marking the customer as verified in the widget.",
    "/developers#identity"
  ],
  [
    "Does verification give an agent access to all customer data?",
    "No. Identity verification does not grant tool permissions. You select the tools an agent may use, and your connected systems must enforce access to customer data.",
    "/developers#identity"
  ]
] as const;

function Actions() {
  return <div className="cta-row"><a className="btn btn-primary" href={SIGNUP_URL}>Start free trial <ArrowRight size={16} aria-hidden="true" /></a><a className="btn btn-secondary" href={DEMO_URL}>Book a demo <ArrowRight size={16} aria-hidden="true" /></a></div>;
}

export default function CustomerSupportPage() {
  return <>
    <PreviewNav tone="dark" />
    <div className="support-page">
      <section className="hero support-hero motion-hero"><HeroVortex variant="flow" tone="dark" />
        <div className="wrap">

          <div className="support-hero-grid">
            <div className="hero-inner">
              <Availability category="Customer support for SaaS teams" />
              <h1>AI agents that help customers.<br /><span>And handle what comes next.</span></h1>
              <p className="lede">The Echo agent answers from your latest docs, past conversations, and connected tools. It can handle everyday questions and follow-ups, and bring your team in when needed.</p>
              <Actions />
              <p className="cta-note">14-day free trial · No card required · No per-seat or per-resolution fees</p>
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>



      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="The shared inbox" title="Pick up where the last conversation left off." lede="Handle support, billing, and sales email in shared team inboxes. People and agents see earlier replies, customer details, and linked work without asking the customer to explain again." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-operations" className="support-operations section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Everyday support" title="Organize customer messages and get them to the right team." lede="Keep your email address. Route chat and email to the right team, automate routine answers with AI agents, and see which conversations still need a person." />
          <SupportInboxFeatures />
          <h3 className="support-tools-heading">Less chasing. More useful replies.</h3>
          <div className="support-features support-operation-tools">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-knowledge" className="support-knowledge-section">
        <div className="wrap support-knowledge-grid">
          <SectionHead eyebrow="Learn from unanswered questions" title="Find what keeps customers stuck." lede="Coverage gaps brings related questions together. See where customers need a clearer guide, your team needs a process, or the product needs a fix. The Quill agent can prepare the missing guidance for review." />
          <WorkScene variant="coverage" />
        </div>
      </section>

      <section id="support-workflow" className="support-workflow-section">
        <div className="wrap">
          <SectionHead eyebrow="AI and your team" title="Answer, hand over, and follow through." lede="The Echo agent checks the available knowledge and previous attempts. If a teammate needs to step in, the findings go with the conversation. Everyone can see what still needs doing." />
          <div className="support-workflow-grid">
            <article><div className="support-step-copy"><span className="support-step-number">01 / ANSWER</span><h3>Check before giving an answer.</h3><p>The Echo agent checks your selected docs and permitted account tools. It can reply directly or prepare an answer for your team.</p></div><SupportScene variant="answer" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">02 / HAND OFF</span><h3>Give the next person a head start.</h3><p>Keep earlier replies, attempted fixes, and the agent’s findings in the same thread.</p></div><SupportScene variant="handoff" /></article>
            <article><div className="support-step-copy"><span className="support-step-number">03 / FOLLOW THROUGH</span><h3>Tell the people waiting for the fix.</h3><p>Link affected conversations to the task. Configure an agent follow-up after the release is confirmed, with the approvals you choose.</p></div><SupportScene variant="followup" /></article>
          </div>
        </div>
      </section>


      <section id="support-connected" className="support-connected-section">
        <div className="wrap support-connected-grid">
          <div><SectionHead eyebrow="Beyond the inbox" title="AI agents fix bugs. Your team reviews the PRs." lede="Send a customer’s bug report to Helpin AI. Scribe plans the change, Forge prepares the fix and opens a pull request, and your team reviews it before merging." secondaryLede="A connected repository and Agent Runtime are required." /><Link prefetch={false} className="btn-link" href="/#ask-agent">Explore Helpin AI <ArrowRight size={16} aria-hidden="true" /></Link></div>
          <figure className="support-connected-preview"><SupportWorkspace variant="agent" /><figcaption className="support-demo-caption">An agent prepares a fix with the customer’s original report attached.</figcaption></figure>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Your team sets the rules" title="Keep control of how your agents work." lede="Choose direct replies or private assistance. Give agents only the tools they need, verify signed-in customers, and decide which actions require approval." />
          <SupportControls />
          <p className="support-controls-link"><Link prefetch={false} className="btn-link" href="/developers#identity">How identity verification and tool access work <ArrowRight size={16} aria-hidden="true" /></Link></p>
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s Support tool" /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="btn-link" href="/self-hosting">Explore self-hosting <ArrowRight size={16} aria-hidden="true" /></Link></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">14-day free trial</span><h2 id="support-final-cta-title">Give customers an answer.<br />Keep the next step moving.</h2><p className="lede">Put the Echo agent and your team in one inbox. Unlimited teammates, included AI usage, and no per-resolution fees.</p><Actions /><p className="cta-note">No card required · Self-host free, or let us run it</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
