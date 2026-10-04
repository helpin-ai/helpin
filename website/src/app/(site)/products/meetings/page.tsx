import { HeroVortex } from '../../_components/HeroVortex';
import {  DEMO_URL, FAQList } from '../../_components/ui';
import { createPageMetadata, PAGE_SEO } from '@/lib/metadata';
import Link from 'next/link';
import { ArrowRight, CalendarDays, Mic, Settings2 } from 'lucide-react';
import { PreviewNav } from '../../_components/PreviewNav';
import { PreviewFooter } from '../../_components/PreviewFooter';
import { ConnectedWorkspace } from '../../_components/ConnectedWorkspace';
import { CtaRow, SectionHead } from '../../_components/ui';
import { MeetingScene, MeetingEvidence } from './meeting-scenes';
import './meetings.css';
import { MeetingWorkspace } from './meeting-workspace';
import { MeetingAgenda } from './meeting-agenda';
import { MeetingContext } from './meeting-context';

export const metadata = createPageMetadata(PAGE_SEO.meetings);
const PLATFORMS = [['google_meet', 'Google Meet'], ['zoom', 'Zoom'], ['teams', 'Microsoft Teams'], ['webex', 'Webex']];
const FAQS = [
  [
    "How does Helpin capture meetings?",
    "A notetaker bot joins the call as a participant and captures the transcript, plus video when recording is on. There is no desktop app to install, and uploading existing recordings isn’t supported.",
    "/products/meetings#meeting-capture"
  ],
  [
    "Which meeting platforms can Helpin join?",
    "Google Meet, Zoom, Microsoft Teams, and Webex. The host may need to admit the notetaker.",
    "/products/meetings#meeting-capture"
  ],
  [
    "Can I choose which meetings are captured?",
    "Yes. Connect Google Calendar, then pick individual calls, a whole recurring series, or a default: every call, or calls with CRM contacts. You can also paste a meeting link.",
    "/products/meetings#meeting-capture"
  ],
  [
    "What do I get after a meeting?",
    "A speaker-attributed transcript, summary, decisions, open questions, action items, and a follow-up draft. Playback requires recording to be enabled.",
    "/products/meetings#meeting-decisions"
  ],
  [
    "Can action items become project tasks?",
    "Yes. Pick a team and status, and Helpin creates a task linked to the meeting.",
    "/products/meetings#meeting-work"
  ],
  [
    "Are follow-up emails sent automatically?",
    "No. Helpin drafts the email. Your team reviews it and sends it from the connected Gmail account.",
    "/products/meetings#meeting-work"
  ],
  [
    "Does the meeting stay connected to the customer?",
    "Yes. Meetings link automatically to the contacts, company, and deal from the calendar invite, and you can add or remove links.",
    "/products/meetings#meeting-context"
  ],
  [
    "Can we control the notetaker and recordings?",
    "Yes. Configure its name, joining preferences, and recording settings.",
    "/products/meetings#meeting-capture"
  ],
  ["Can we self-host Meetings?", "Yes. Meetings is included in Helpin Community, free under AGPL-3.0 with no plan limits. Community is in 0.2 beta. Capture needs a Recall.ai account or a Vexa deployment (hosted or self-hosted); Webex requires Recall.ai. Calendar sync needs a Google OAuth app, and summaries need an AI provider.", "/self-hosting#whats-included"]
] as const;
export default function MeetingsPage() {
  return <><PreviewNav tone="dark" /><main className="meetings-page">
    <section className="meetings-hero motion-hero" aria-labelledby="meetings-title"><HeroVortex variant="flow" tone="dark" /><div className="wrap">

      <div className="meetings-hero-copy"><span className="eyebrow">Meetings</span><h1 id="meetings-title">AI takes the notes.<br /><span>Put your agents to work.</span></h1><p className="lede">Capture team calls, customer demos, and planning discussions. Get the notes and action items, then use Ask Agent to turn what was discussed into a clearer plan and useful follow-up.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="meetings-supporting-note">14-day free trial · No card required</p><div className="meetings-platforms">{PLATFORMS.map(([id,label])=><span key={id}><img src={`/new/meetings/${id}.svg`} width={19} height={19} alt=""/>{label}</span>)}</div></div>
      <div className="meetings-screenshot"><MeetingWorkspace /></div>
    </div></section>

    <section id="meeting-capture"><div className="wrap"><div className="meetings-split"><div><SectionHead eyebrow="Choose what gets captured" title="Capture the calls that matter." lede="Connect Google Calendar or paste a meeting link. Choose internal meetings, customer calls, or a recurring series. The notetaker joins the calls you select."/></div><MeetingAgenda/></div><div className="meetings-features">{[
      {Icon:CalendarDays,title:'One call or the whole series.',body:'Choose an individual meeting, enable a recurring series, or skip a specific occurrence.'},
      {Icon:Settings2,title:'Your notetaker. Your preferences.',body:'Set its name. Join calls manually, join every call, or join only calls with CRM contacts.'},
      {Icon:Mic,title:'Keep playback when you need it.',body:'Choose whether recordings are saved, with a default setting or a preference for an individual meeting.'},
    ].map(({Icon,title,body})=><article key={title}><Icon size={21}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="meeting-decisions" className="meetings-dark meetings-insights-dark section-motion"><HeroVortex variant="converge" tone="dark" /><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="From transcript to understanding" title="Keep the decision. Check what is still open." lede="AI separates decisions, action items, and open questions. Check the speaker and transcript before turning a suggestion into a commitment."/></div><MeetingEvidence/></div></section>
    <section id="meeting-work" className="meetings-work-light"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="Put the decisions to work" title="Give the action item somewhere to go." lede="Create a task from the discussion, choose the team, and give the work an owner. Ask Agent can use the meeting when helping plan the next step."/></div><div className="meetings-work-grid"><article><div className="meetings-work-copy"><span className="meetings-number">01 / ASSIGN THE WORK</span><h3>Turn the action item into a task.</h3><p>Pick a team and status, then create a task linked to the meeting.</p></div><MeetingScene variant="task"/><p className="meetings-demo-caption">A task your team can track, with the reason behind it attached.</p></article><article><div className="meetings-work-copy"><span className="meetings-number">02 / PREPARE THE FOLLOW-UP</span><h3>Prepare the follow-up while it is fresh.</h3><p>Helpin drafts the follow-up from the discussion. Review the wording and commitments, then send it from your connected Gmail account.</p></div><MeetingScene variant="followup"/><p className="meetings-demo-caption">The agreed next steps, ready for your review.</p></article></div><Link prefetch={false} className="meetings-inline-link" href="/products/projects">Explore Projects<ArrowRight size={16}/></Link></div></section>
    <section id="meeting-context" className="meetings-context-section"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="Useful after the call" title="Ask what happened after the meeting." lede="Ask Agent to find the agreement and check linked work before the next call. Customer meetings connect to the account. Internal discussions stay useful when the team needs to revisit a decision."/><Link prefetch={false} className="meetings-inline-link" href="/products/ai-agents">Explore Ask Agent<ArrowRight size={16}/></Link></div><MeetingContext/></div></section>
    <section id="meeting-faq" className="meetings-soft"><div className="wrap meetings-faq-grid"><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s Meetings tool"/><div className="meetings-faqs"><FAQList items={FAQS} className="faq-items" /><Link prefetch={false} className="meetings-inline-link" href="/self-hosting">Explore self-hosting<ArrowRight size={16}/></Link></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="meetings-final-title"><div className="wrap"><ConnectedWorkspace/><div className="final"><span className="eyebrow">From conversation to follow-through</span><h2 id="meetings-final-title">Leave the call<br/>with a clearer next step.</h2><p className="lede">Keep the notes, assign the work, and ask Helpin AI to help your team follow through.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="meetings-supporting-note">14-day free trial · No card required</p><p className="meetings-supporting-note">Open source · Self-host free, or let us run it</p></div></div></section>
  </main><PreviewFooter homepage/></>;
}
