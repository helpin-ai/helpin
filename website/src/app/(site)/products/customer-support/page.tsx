import { HeroVortex } from '../../_components/HeroVortex';
import { Availability, DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, MessageSquare, Clock3, Paperclip, Search, Sparkles, Star, Users, Zap } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { SectionHead, SIGNUP_URL } from '../../_components/ui';
import { CoverageGapsScene } from '../../_components/docs-paths/DocsPaths';
import { ImportScene } from '../../_components/import-scene/ImportScene';
import { SupportScene } from './support-scene';
import { SupportControls } from './support-controls';
import { SupportChannels } from './support-channels';
import { SupportInboxFeatures } from './support-inbox-features';
import { SupportInboxShowcase } from './support-inbox-showcase';
import { SupportHeroScene } from './support-hero-scene';
import { PortalScene, ReportsCard, SlaScene } from './support-desk-scenes';
import './support.css';
import { DOCS } from '../../_components/docsLinks';

export const metadata = createPageMetadata(PAGE_SEO.customerSupport);

const FEATURES = [
  { icon: MessageSquare, title: 'Internal notes and mentions', copy: 'Discuss the next step privately and bring teammates into the conversation.' },
  { icon: Zap, title: 'Saved replies', copy: 'Reuse common answers, then personalize them before sending.' },
  { icon: Users, title: 'Team presence', copy: 'See who’s viewing or typing before you reply.' },
  { icon: Search, title: 'Conversation search', copy: 'Find an earlier question without opening every thread.' },
  { icon: Clock3, title: 'Office hours and reply times', copy: 'Let customers know when to expect your team. The same business hours can drive SLA clocks.' },
  { icon: Paperclip, title: 'Files and attachments', copy: 'Keep screenshots and supporting files with the request.' },
  { icon: Sparkles, title: 'AI follow-ups', copy: 'The Echo agent can check whether the answer helped when a customer goes quiet. Cancel a pending follow-up whenever you need to.' },
  { icon: Star, title: 'Satisfaction ratings', copy: 'Customers rate resolved conversations in the chat widget or the portal, with an optional comment.' },
];

const FAQS = [
  [
    "Which channels can customers use?",
    "Chat on your site or app, email to your existing support address, Telegram through a connected bot, and the customer portal. All of them arrive in the same shared inbox, and replies go back on the channel the customer used.",
    "#support-channels"
  ],
  [
    "Can customers reach us on Telegram?",
    "Yes. Connect a Telegram bot with its token from @BotFather. New conversations go to the team inbox you choose, routing rules can still move them, and you decide whether AI replies to that bot. Telegram is the only messaging app supported today.",
    "#support-channels"
  ],
  [
    "Do we need a new support email address?",
    "No. Keep your existing address and verify it for replies through Helpin.",
    DOCS.senderAddresses
  ],
  [
    "Can we reply to customers in another language?",
    "Yes. Turn on Live Translate for a conversation. Helpin AI translates new customer messages into your reading language and sends your replies in theirs. You can choose the customer’s language, pause translation and view the original text.",
    "#support-live-translate",
    "See Live Translate"
  ],
  [
    "How are conversations assigned?",
    "Route requests to team inboxes with routing rules or AI triage, then assign owners manually or with round-robin assignment. An SLA policy can also assign a conversation when its warning fires and nobody owns it yet.",
    "#support-channels"
  ],
  [
    "Do SLA targets respect business hours?",
    "Yes, when the policy counts business hours only. Clocks then stop outside the business hours set in Chat widget settings. Without business hours, targets run on calendar time. A policy can also pause the resolution clock while you wait on the customer.",
    "#support-sla"
  ],
  [
    "Who can see requests in the customer portal?",
    "Requesters see only their own requests. Company members see every request from their company, and company admins can also invite colleagues and manage who has access. Customers can copy colleagues on a request.",
    "#support-portal"
  ],
  [
    "Can customers rate the support they received?",
    "Yes. After a conversation is resolved, customers can rate it in the chat widget or the portal and add a comment. Reports show the average rating for conversations resolved by AI and by your team.",
    "#support-reports"
  ],
  [
    "What information can agents use to answer?",
    "The Echo agent uses the docs, website pages, and files you select. It can also use relevant conversations and permitted connected tools to check account facts. Your systems control which customer data those tools return.",
    "/products/knowledge"
  ],
  [
    "Do agents have to send replies automatically?",
    "No. Your team can use AI for internal assistance instead, or turn automatic responses off.",
    "#support-controls"
  ],
  [
    "What does a teammate receive during a handoff?",
    "The customer’s messages, earlier replies, and the agent’s handoff notes stay in the conversation.",
    "#support-inbox"
  ],
  [
    "Can we import from Intercom or Zendesk?",
    "Yes for Intercom. Import your Help Center and past conversations with their replies, notes, and tags, or copy a public Intercom help center from its address. There is no Zendesk importer yet; forward your support email to Helpin and new conversations start there.",
    "#support-switch",
    "See importing"
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
              <p className="lede">Customers reach you by chat, email, Telegram, or your portal, and every request lands in one inbox. In chat, the Echo agent answers from your docs, past conversations, and connected tools, follows up when a customer goes quiet, and brings your team in with what it found.</p>
              <Actions />
              <p className="cta-note">14-day free trial · No card required · No per-seat or per-resolution fees</p>
            </div>
            <SupportHeroScene />
          </div>
        </div>
      </section>

      <section id="support-inbox" className="support-inbox-section">
        <div className="wrap">
          <SectionHead eyebrow="The shared inbox" title="Pick up where the last conversation left off." lede="Handle support, billing, and sales conversations in shared team inboxes. People and agents see earlier replies, customer details, and linked work without asking the customer to explain again." />
          <SupportInboxShowcase />
        </div>
      </section>

      <section id="support-channels" className="support-operations section-motion"><HeroVortex variant="converge" tone="dark" />
        <div className="wrap">
          <SectionHead eyebrow="Channels" title="Every channel lands in the same inbox." lede="Chat, email, Telegram, and portal requests arrive as conversations in your team inboxes, with the customer’s history attached. Reply from Helpin and the answer goes back on the channel the customer used." />
          <SupportChannels />
          <SupportInboxFeatures />
          <h3 className="support-tools-heading">Also in every conversation</h3>
          <div className="support-features support-operation-tools">{FEATURES.map(({ icon: Icon, title, copy }) => <article key={title}><Icon size={23} strokeWidth={1.5} aria-hidden="true" /><h3>{title}</h3><p>{copy}</p></article>)}</div>
        </div>
      </section>

      <section id="support-sla" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="SLAs" title="Response targets your team can see and keep." tight
              lede="Set first-response, next-response, and resolution targets for the conversations that matter most. Helpin shows the time left on every conversation and warns before a target is missed." />
            <p>A policy applies by priority, team inbox, channel, request type, or the customer’s company and its SLA tier. Clocks can count business hours only, and the resolution clock can pause while you wait on the customer.</p>
            <p>Before a breach, Helpin warns your team and anyone you add, including the company’s customer success owner. If nobody owns the conversation yet, the policy can assign it. Warnings and breaches are also sent as webhooks.</p>
          </div>
          <SlaScene />
        </div>
      </section>

      <section id="support-portal" className="support-desk-section support-desk-section-tint">
        <div className="wrap support-desk-grid support-desk-grid-flip">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Customer portal" title="A place for customers to follow their requests." tight
              lede="Customers sign in with an emailed link to open requests, follow every reply, and search their history. An assistant answers from your help articles, with links to the sources, before they ask." />
            <p>Request types ask the right questions up front, and the same types can set SLA targets. Customers attach files and copy colleagues, and related help articles appear while they write.</p>
            <p>Requesters see their own requests. Company members see every request from their company, and company admins invite colleagues. When a request is resolved, customers rate the support they received.</p>
            <Link prefetch={false} className="btn-link" href="/products/customer-portal">Explore the customer portal <ArrowRight size={16} aria-hidden="true" /></Link>
          </div>
          <PortalScene />
        </div>
      </section>

      <section id="support-knowledge" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Coverage gaps" title="Turn repeated questions into help articles." tight
              lede="Helpin groups the questions your AI couldn’t answer well into coverage gaps, with how many conversations hit each one in the last 30 days. Similar gaps merge, so one article covers all of them." />
            <p>Open a gap and click <b>Prepare a fix</b>. Quill, the documentation agent, reads the conversations, checks your existing guides, and saves a draft for your team to review.</p>
            <Link prefetch={false} className="btn-link" href="/products/knowledge">Explore knowledge and docs <ArrowRight size={16} aria-hidden="true" /></Link>
          </div>
          <CoverageGapsScene />
        </div>
      </section>

      <section id="support-reports" className="support-desk-section support-desk-section-tint">
        <div className="wrap support-desk-grid support-desk-grid-flip">
          <div className="support-desk-copy">
            <SectionHead eyebrow="Reports" title="See what the AI resolves and where customers wait." tight
              lede="Reports cover new and resolved conversations, response times, how often the AI resolves a conversation or hands it to your team, satisfaction ratings, and SLA targets met." />
            <p>Compare ratings for conversations resolved by AI and by your team, open the conversations behind a missed target, and see volume by channel, topic, and teammate.</p>
          </div>
          <ReportsCard />
        </div>
      </section>

      <section id="support-workflow" className="support-desk-section">
        <div className="wrap support-desk-grid">
          <div className="support-desk-copy">
            <SectionHead eyebrow="When it’s a bug" title="Tell everyone who’s waiting for the fix." tight
              lede="Link the affected conversations to the task. After the release is confirmed, the Echo agent can follow up with each customer, with the approvals you choose." />
            <p>AI agents can also plan the change and prepare the fix in a connected repository, with the customer’s report attached. Your team reviews it before anything merges.</p>
            <Link prefetch={false} className="btn-link" href="/products/ai-agents">Explore AI agents <ArrowRight size={16} aria-hidden="true" /></Link>
          </div>
          <div className="support-desk-followup"><SupportScene variant="followup" /></div>
        </div>
      </section>

      <section id="support-controls" className="support-controls-section">
        <div className="wrap">
          <SectionHead eyebrow="Your team sets the rules" title="Keep control of how your agents work." lede="Choose direct replies or private assistance. Give agents only the tools they need, verify signed-in customers, and decide which actions require approval." />
          <SupportControls />
          <p className="support-controls-link"><Link prefetch={false} className="btn-link" href="/developers#identity">How identity verification and tool access work <ArrowRight size={16} aria-hidden="true" /></Link></p>
        </div>
      </section>

      <section id="support-switch" className="switch-section">
        <div className="wrap switch-grid">
          <div className="switch-copy">
            <SectionHead eyebrow="Switching" title="Move from Intercom without starting over." tight
              lede="Bring your help center and your past conversations, with their replies, notes, and tags. Your AI agents answer with that history from the first day." />
            <div className="switch-sources">
              <article><h3>Intercom</h3><p>Conversations and the Help Center, from the data region you use and from the date you choose. Articles keep their Intercom status or come in as drafts.</p></article>
              <article><h3>Other help centers</h3><p>Copy a public Intercom help center from its address, with no token. Import HelpScout Docs or a Nextra site with images and redirects.</p></article>
            </div>
            <Link prefetch={false} className="btn-link" href="/compare/intercom">See how Helpin compares with Intercom →</Link>
          </div>
          <ImportScene />
        </div>
      </section>

      <section id="support-faq" className="support-faq-section">
        <div className="wrap support-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s Support tool" /><div className="support-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="btn-link" href="/self-hosting">Explore self-hosting <ArrowRight size={16} aria-hidden="true" /></Link></div></div>
      </section>

      <section className="final-cta final-cta-connected" aria-labelledby="support-final-cta-title"><div className="wrap"><ConnectedWorkspace focus="support" /><div className="final"><span className="eyebrow">14-day free trial</span><h2 id="support-final-cta-title">Give customers an answer.<br />Keep the next step moving.</h2><p className="lede">Put the Echo agent and your team in one inbox. Unlimited teammates, included AI usage, and no per-resolution fees.</p><Actions /><p className="cta-note">No card required · Self-host free, or let us run it</p></div></div></section>
    </div>
    <PreviewFooter homepage />
  </>;
}
