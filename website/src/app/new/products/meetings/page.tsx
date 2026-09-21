import { Availability, CtaNote, DEMO_URL, FAQList } from '../../_components/ui';
import { previewMetadata } from '../../_components/preview-metadata';
import Link from 'next/link';
import { ArrowRight, CalendarDays, CheckCheck, ChevronRight, FileText, Mic, Settings2 } from 'lucide-react';
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
    "Helpin recognizes Google Meet, Zoom, Microsoft Teams and Webex links. Your capture provider and the meeting’s access settings determine whether the notetaker can join.",
    "/new/products/meetings#meeting-capture"
  ],
  [
    "Can I choose which calls are captured?",
    "Yes. Capture a single meeting, choose recurring meetings, or set defaults for calls with external attendees.",
    "/new/products/meetings#meeting-capture"
  ],
  [
    "What do I get after a meeting?",
    "A transcript, summary, decisions and suggested action items, plus a follow-up draft. Recordings are available when recording is enabled.",
    "/new/products/meetings#meeting-decisions"
  ],
  [
    "Can action items become project tasks?",
    "Yes. Review an action item, choose its team and owner, and create a task with the meeting attached.",
    "/new/products/meetings#meeting-work"
  ],
  [
    "Are follow-up emails sent automatically?",
    "No. Helpin prepares a draft for your team to review before sending.",
    "/new/products/meetings#meeting-work"
  ],
  [
    "Does the meeting stay attached to the customer?",
    "Yes. Attach meetings to contacts, companies, deals and tasks so the next teammate can find the discussion.",
    "/new/products/meetings#meeting-context"
  ],
  [
    "Can we control recordings and the notetaker?",
    "Yes. Choose the notetaker name, join preferences and whether recordings are saved.",
    "/new/products/meetings#meeting-capture"
  ],
  ["Can we self-host Meetings?", "Yes. Meetings is part of the open-source product. Run it on your own infrastructure or use Helpin Cloud.", "/new/self-hosting#whats-included"]
] as const;
export default function MeetingsPage() {
  return <><PreviewNav /><main className="meetings-page">
    <section className="meetings-hero" aria-labelledby="meetings-title"><div className="wrap">
      <div className="meetings-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12}/><span>Meetings</span></div>
      <div className="meetings-hero-copy"><Availability category="Meeting notes" /><h1 id="meetings-title">Turn customer calls into <span>tasks and follow-ups.</span></h1><p className="lede">Let Helpin capture the call, pull out the decisions, and prepare the next steps. Review the task or follow-up with the customer’s words still in view.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /><div className="meetings-platforms">{PLATFORMS.map(([id,label])=><span key={id}><img src={`/new/meetings/${id}.svg`} width={19} height={19} alt=""/>{label}</span>)}</div></div>
      <div className="meetings-screenshot"><MeetingWorkspace /></div>
    </div></section>
    <nav className="meetings-page-nav" aria-label="On this page"><div className="wrap"><strong>Meetings</strong><a href="#meeting-capture">Capture the call</a><a href="#meeting-decisions">Find the decisions</a><a href="#meeting-work">Move work forward</a><a href="#meeting-context">Keep the context</a></div></nav>
    <section id="meeting-capture"><div className="wrap"><div className="meetings-split"><div><SectionHead eyebrow="Choose the calls that matter" title="Stay in the conversation. Helpin keeps the details." lede="See upcoming calls in one list and choose which ones Helpin joins. Turn on a recurring series, skip an individual date, or add a meeting link yourself." secondaryLede="Stay with the customer’s questions. Helpin captures the conversation so your team can revisit the decisions and next steps."/></div><MeetingAgenda/></div><div className="meetings-features">{[
      {Icon:CalendarDays,title:'Choose the calls that matter.',body:'Capture one meeting, set a recurring-series preference, or use workspace defaults for eligible calendar events.'},
      {Icon:Settings2,title:'Make the notetaker yours.',body:'Set its name and join behavior. Choose manual capture, external meetings, or all eligible meetings.'},
      {Icon:Mic,title:'Keep recordings when you need them.',body:'Choose whether to save recordings for playback. Set a default or a preference for an individual meeting.'},
    ].map(({Icon,title,body})=><article key={title}><Icon size={21}/><h3>{title}</h3><p>{body}</p></article>)}</div></div></section>
    <section id="meeting-decisions" className="meetings-dark meetings-insights-dark"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="More than a transcript" title="Find the decision, then see what was said." lede="Read the summary, review open questions, and check action items against the conversation. Speaker names and timestamps help you return to the moment that matters."/></div><MeetingEvidence/><div className="meetings-detail-row"><span><Mic size={16}/>Speaker-attributed transcripts</span><span><FileText size={16}/>Decisions, risks & open questions</span><span><CheckCheck size={16}/>Action items with evidence</span></div></div></section>
    <section id="meeting-work" className="meetings-work-light"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="From a promise to a next step" title="Turn what you agreed into work your team owns." lede="A promise in the transcript needs an owner and a destination. Review the action item, create the linked task, and check the follow-up before it reaches the customer."/></div><div className="meetings-work-grid"><article><div className="meetings-work-copy"><span className="meetings-number">01 / ASSIGN THE WORK</span><h3>Turn a commitment into a task.</h3><p>Choose the team, owner, and workflow state. The task stays linked to the meeting that started it.</p></div><MeetingScene variant="task"/></article><article><div className="meetings-work-copy"><span className="meetings-number">02 / PREPARE THE FOLLOW-UP</span><h3>Leave the customer with a clear next step.</h3><p>Start from a draft grounded in the discussion. Review the wording and commitments before following up.</p></div><MeetingScene variant="followup"/></article></div><Link className="meetings-inline-link" href="/new/products/projects">See how Projects keeps work moving<ArrowRight size={16}/></Link></div></section>
    <section id="meeting-context" className="meetings-context-section"><div className="wrap"><div className="meetings-centered"><SectionHead eyebrow="Part of the customer’s history" title="Give the next teammate the whole conversation." lede="Keep the meeting connected to the customer and the work. Ask Agent can use that context to brief your teammate on what was agreed, what is still open, and who owns the next step."/><Link className="meetings-inline-link" href="/new/products/ai-agents">Explore your agents<ArrowRight size={16}/></Link></div><MeetingContext/></div></section>
    <section id="meeting-faq" className="meetings-soft"><div className="wrap meetings-faq-grid"><SectionHead eyebrow="Questions" title="What to know before your next call."/><div className="meetings-faqs"><FAQList items={FAQS} className="faq-items" /></div></div></section>
    <section className="final-cta final-cta-connected" aria-labelledby="meetings-final-title"><div className="wrap"><ConnectedWorkspace/><div className="final"><span className="eyebrow">Keep the conversation moving</span><h2 id="meetings-final-title">Make the next customer call count.</h2><p className="lede">Bring the conversation, the decisions, and the work that follows into one workspace.</p><CtaRow secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><CtaNote trial /></div></div></section>
  </main><PreviewFooter/></>;
}
