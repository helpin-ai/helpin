import { HeroVortex } from '../../_components/HeroVortex';
import {  DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, CalendarDays, ChevronRight, Mic, Settings2 } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { MeetingScene, MeetingEvidence } from './meeting-scenes';
import './meetings.css';
import { MeetingWorkspace } from './meeting-workspace';
import { MeetingAgenda } from './meeting-agenda';
import { MeetingContext } from './meeting-context';

export const metadata = previewMetadata("Meetings \u2014 Helpin", "/new/products/meetings");
const PLATFORMS = [['google_meet', 'Google Meet'], ['zoom', 'Zoom'], ['teams', 'Microsoft Teams'], ['webex', 'Webex']];
const FAQS = [
  [
    "Which meeting platforms can Helpin join?",
    "Google Meet, Zoom, Microsoft Teams, and Webex. Successful joining depends on the configured capture provider and meeting access.",
    "/new/products/meetings#meeting-capture"
  ],
  [
    "Can I choose which meetings are captured?",
    "Yes. Select individual calls, recurring series, or defaults for eligible meetings.",
    "/new/products/meetings#meeting-capture"
  ],
  [
    "What do I get after a meeting?",
    "A transcript, summary, decisions, suggested actions, and follow-up draft. Playback requires recording to be enabled.",
    "/new/products/meetings#meeting-decisions"
  ],
  [
    "Can action items become project tasks?",
    "Yes. Review an action item and create a linked task.",
    "/new/products/meetings#meeting-work"
  ],
  [
    "Are follow-up emails sent automatically?",
    "No. Your team reviews the draft before sending.",
    "/new/products/meetings#meeting-work"
  ],
  [
    "Does the meeting stay connected to the customer?",
    "Yes. Link meetings with the relevant customer records and work, so the discussion remains available alongside the account’s other activity.",
    "/new/products/meetings#meeting-context"
  ],
  [
    "Can we control the notetaker and recordings?",
    "Yes. Configure its name, joining preferences, and recording settings.",
    "/new/products/meetings#meeting-capture"
  ],
  ["Can we self-host Meetings?", "Yes. Meetings is included in Helpin’s open-source product. You operate the installation and configure the services it uses, including meeting capture and AI providers. Hosting and provider charges may apply.", "/new/self-hosting#whats-included"]
] as const;
export default function MeetingsPage() {
  return <><PreviewNav /><main className="meetings-page">
    <section className="meetings-hero motion-hero" aria-labelledby="meetings-title"><HeroVortex variant="flow" tone="dark" /><div className="wrap">
      <div className="meetings-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12}/><span>Meetings</span></div>
      <div className="meetings-hero-copy"><span className="eyebrow">Meetings for SaaS teams</span><h1 id="meetings-title">AI agents that turn meeting decisions <span>into next steps.</span></h1><p className="lede">Capture customer calls, review what was agreed, and prepare tasks and follow-ups. Helpin keeps the conversation connected to the customer and the work—so your team and agents can pick up where the meeting left off.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="meetings-supporting-note">14-day cloud trial · No credit card required.</p><div className="meetings-platforms">{PLATFORMS.map(([id,label])=><span key={id}><img src={`/new/meetings/${id}.svg`} width={19} height={19} alt=""/>{label}</span>)}</div><p className="meetings-supporting-note">Joining depends on your capture provider and the meeting’s access settings.</p></div>
      <div className="meetings-screenshot"><MeetingWorkspace /><p className="meetings-demo-caption">The discussion, the decision, and the next step—together.</p></div>
    </div></section>
    <nav className="meetings-page-nav" aria-label="On this page"><div className="wrap"><strong>Meetings</strong><a href="#meeting-capture">Capture the call</a><a href="#meeting-decisions">Find the decisions</a><a href="#meeting-work">Move work forward</a><a href="#meeting-context">Keep the context</a></div></nav>
    <section id="meeting-capture"><div className="wrap"><div className="meetings-split"><div><SectionHead eyebrow="Choose what gets captured" title="Be in the conversation. Keep the details for later." lede="Choose the calls Helpin joins from your upcoming meetings, or add a meeting link. Capture the discussion now so your team can return to it when it is time to act."/></div><MeetingAgenda/></div><div className="meetings-features">{[
      {Icon:CalendarDays,title:'One call or the whole series.',body:'Choose an individual meeting, enable a recurring series, or skip a specific occurrence.'},
      {Icon:Settings2,title:'Your notetaker. Your preferences.',body:'Set its name and join behavior. Capture meetings manually or use defaults for eligible calls.'},
      {Icon:Mic,title:'Keep playback when you need it.',body:'Choose whether recordings are saved, with a default setting or a preference for an individual meeting.'},
    ].map(({Icon,title,body})=><article key={title}><Icon size={21}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="meeting-decisions" className="meetings-dark meetings-insights-dark"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="From transcript to understanding" title="Know what was agreed. See what still needs an answer." lede="Review the summary, decisions, and suggested actions. Check the speaker and timestamp before treating a suggestion as a commitment."/></div><MeetingEvidence/><p className="meetings-demo-caption">Understand the decision before you act on it.</p></div></section>
    <section id="meeting-work" className="meetings-work-light"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="Put the decisions to work" title="Give the next step an owner and a place to happen." lede="Bring meeting commitments into the projects your team already manages. Keep the original discussion close as the work is assigned, prioritized, and completed."/></div><div className="meetings-work-grid"><article><div className="meetings-work-copy"><span className="meetings-number">01 / ASSIGN THE WORK</span><h3>Move the commitment out of the notes.</h3><p>Review the suggested action, choose its team and owner, and create a task linked to the meeting.</p></div><MeetingScene variant="task"/><p className="meetings-demo-caption">A task your team can track, with the reason behind it attached.</p></article><article><div className="meetings-work-copy"><span className="meetings-number">02 / PREPARE THE FOLLOW-UP</span><h3>Send the next step, not just a recap.</h3><p>Prepare a follow-up from the discussion. Check the wording and commitments before sending it to the customer.</p></div><MeetingScene variant="followup"/><p className="meetings-demo-caption">The agreed next steps, ready for your review.</p></article></div><Link className="meetings-inline-link" href="/new/products/projects">Explore Projects<ArrowRight size={16}/></Link></div></section>
    <section id="meeting-context" className="meetings-context-section"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="Part of the customer’s history" title="Know what was promised. See where the work stands." lede="Connect the meeting to the customer, deal, and related work. Ask Agent can bring that history together before the next call—so your teammate sees the agreement and the progress behind it."/><Link className="meetings-inline-link" href="/new/products/ai-agents">Explore Ask Agent<ArrowRight size={16}/></Link></div><MeetingContext/><p className="meetings-demo-caption">The next conversation starts with what your team already knows.</p></div></section>
    <section id="meeting-faq" className="meetings-soft"><div className="wrap meetings-faq-grid"><SectionHead eyebrow="Before your next meeting" title="A few things worth knowing."/><div className="meetings-faqs"><FAQList items={FAQS} className="faq-items" /><Link className="meetings-inline-link" href="/new/self-hosting">Explore self-hosting<ArrowRight size={16}/></Link></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="meetings-final-title"><div className="wrap"><ConnectedWorkspace/><div className="final"><span className="eyebrow">From conversation to follow-through</span><h2 id="meetings-final-title">Make the next step<br/>part of the meeting.</h2><p className="lede">Bring customer calls, decisions, and follow-ups into one workspace. Give your team and AI agents the history to pick up the work—and keep it moving.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="meetings-supporting-note">14-day cloud trial · No credit card required.</p></div></div></section>
  </main><PreviewFooter homepage/></>;
}
