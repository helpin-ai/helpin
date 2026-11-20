'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
  ArrowUpRight, AtSign, BarChart3, Bell, Bold, BookOpen, Bot, Building2, Check, CheckCheck, ChevronDown, ChevronRight, ChevronsUpDown,
  CircleCheck, CircleX, Clock3, Code2, Columns2, FileText, FolderKanban, Globe, Inbox, Italic, Link2, List, ListOrdered, Mail, MessageCircle,
  MoreHorizontal, MousePointer2, Paperclip, Pause, Play, Plus, Quote, Search, Settings, SlidersHorizontal, Smile, Sparkles, Tag, Underline, UserRound, Workflow,
} from 'lucide-react';
import { useWorkflowPlayback } from '../useWorkflowPlayback';
import './support-story.css';

// One scripted support conversation, drawn at the app's own 1600×1000 layout and
// scaled to fit. Stages:
//  0 question · 1 AI answers from docs · 2 follow-up · 3 AI holds and starts a research agent
//  4 research result and answer · 5 bug report, auto classification · 6 AI handoff and assignment
//  7 teammate works another conversation · 8 SLA reminder · 9 back on the conversation
// 10 Ask Agent explains the cause · 11 teammate's reply drafted · 12 reply sent, SLA met
const BEATS = [0, 1400, 4600, 6200, 9200, 13400, 16200, 21000, 23000, 25400, 27400, 32600, 35400];
const DURATION = 43000;
const W = 1600;
const H = 1000;
const MOBILE_W = 420;
const MOBILE_H = 820;

const MAYA = '/new/avatars/maya.webp';
const SAM = '/new/home/teammate-sam.svg';
const REPLY = 'Hi Maya, Sam here. I found the cause: exports over 10,000 rows stop after the first page, so 8,400 of your contacts were left out. For this month’s report, run two exports split by Created date and both files will be complete. The fix is tracked in EXP-142 with your report attached, and I’ll update you here when it ships.';

function Photo({ src, size = 30, label }: { src: string; size?: number; label?: string }) {
  return <img className="hs-photo" src={src} width={size} height={size} alt={label ?? ''} style={{ width: size, height: size }} />;
}
function Initials({ text, tone, size = 34 }: { text: string; tone: number; size?: number }) {
  return <span className="hs-initials" data-tone={tone} style={{ width: size, height: size }}>{text}</span>;
}
function AIMark({ size = 30 }: { size?: number }) {
  return <span className="hs-ai-mark" style={{ width: size, height: size }}><img src="/brand/helpin-icon-white.svg" width={size * 0.56} height={size * 0.56} alt="" /></span>;
}
function TagPill({ name }: { name: 'bug' | 'exports' | 'billing' | 'api' | 'account' | 'onboarding' }) {
  return <span className="hs-tag" data-tag={name}>{name}</span>;
}
function SLA({ label, tone = 'muted' }: { label: string; tone?: 'muted' | 'soon' | 'met' }) {
  return <span className="hs-sla" data-tone={tone}><Clock3 size={10} strokeWidth={2.2} />{label}</span>;
}
function In({ children }: { children: ReactNode }) { return <div className="hs-in">{children}</div>; }

function slaState(stage: number): { label: string; tone: 'muted' | 'soon' | 'met' } | null {
  if (stage < 6) return null;
  if (stage >= 12) return { label: 'NRT met', tone: 'met' };
  if (stage >= 8) return { label: 'NRT 4m', tone: 'soon' };
  if (stage === 7) return { label: 'NRT 9m', tone: 'muted' };
  return { label: 'NRT 15m', tone: 'muted' };
}

function Navigation() {
  const rail: [typeof Inbox, string][] = [[FolderKanban, 'Projects'], [MessageCircle, 'Support'], [FileText, 'Docs'], [Building2, 'CRM'], [Workflow, 'Automation'], [Settings, 'Settings']];
  const queue = (Icon: typeof Inbox, name: string, count?: number, active = false) => <div className="hs-nav-item" data-active={active}><Icon size={15} strokeWidth={1.9} /><span>{name}</span>{count != null && <small>{count}</small>}</div>;
  return <div className="hs-nav">
    <div className="hs-rail">
      <span className="hs-logo">O</span>
      {rail.map(([Icon, name]) => <span key={name} className="hs-rail-item" data-active={name === 'Support'}><span><Icon size={16} strokeWidth={1.9} /></span>{name}</span>)}
      <span className="hs-rail-me"><Photo src={SAM} size={28} /></span>
    </div>
    <div className="hs-menu">
      <div className="hs-workspace"><strong>OrbitDesk</strong><ChevronsUpDown size={14} /><span className="hs-bell"><Bell size={15} /></span><Search size={15} /></div>
      {queue(Inbox, 'Inbox', 24, true)}{queue(UserRound, 'Mine', 3)}{queue(Columns2, 'Waiting', 35)}{queue(Clock3, 'Snoozed')}{queue(CircleCheck, 'Resolved')}{queue(CircleX, 'Spam')}
      <p className="hs-menu-label">HELPIN AI</p>
      {queue(Bot, 'AI Handling', 5)}{queue(CircleCheck, 'AI Resolved')}
      <p className="hs-menu-label">TEAM INBOXES <Plus size={13} /></p>
      <div className="hs-nav-item hs-sub"><span>Billing</span><small>6</small></div>
      <div className="hs-nav-item hs-sub"><span>Technical Support</span><small>4</small></div>
      <div className="hs-nav-item hs-sub"><span>Onboarding</span><small>4</small></div>
      <p className="hs-menu-label">CUSTOM VIEWS <Plus size={13} /></p>
      <div className="hs-nav-item"><Plus size={15} /><span>Create a view</span></div>
      <div className="hs-menu-foot">{queue(BookOpen, 'Coverage Gaps')}{queue(BarChart3, 'Reports')}</div>
    </div>
  </div>;
}

type Row = { name: string; preview: string; time: string; photo?: string; initials?: string; tone?: number; chip?: string; tags?: Parameters<typeof TagPill>[0]['name'][]; mail?: boolean; ai?: boolean };
function ConversationList({ stage, viewing }: { stage: number; viewing: 'maya' | 'alex' }) {
  const sla = slaState(stage);
  const maya: Row = {
    name: 'Maya Chen', time: stage >= 12 ? 'now' : '2m', photo: MAYA,
    preview: stage >= 12 ? 'Hi Maya, Sam here. I found the cause…' : stage >= 5 ? 'Last thing: the export stops at 10,000 rows…' : stage >= 2 ? 'When we open it in Excel, names with…' : 'Does the CSV export include our custom…',
    tags: stage >= 5 ? ['bug', 'exports'] : undefined, ai: stage < 6,
  };
  const rows: Row[] = [
    { name: 'Alex Morgan', preview: 'Could you resend last month’s invoice?', time: '9m', initials: 'AM', tone: 2, chip: 'FRT 3h 57m', tags: ['billing'], mail: true },
    { name: 'Priya Shah', preview: 'Invited the team, thanks for the help!', time: '21m', photo: '/new/avatars/priya.webp', tags: ['onboarding'], ai: true },
    { name: 'Noah Williams', preview: 'Webhook deliveries are retrying twice', time: '28m', initials: 'NW', tone: 1, chip: 'NRT 2h 10m', tags: ['api'] },
    { name: 'Dev Patel', preview: 'Verified, thanks. I’ve reset 2FA for…', time: '56m', photo: '/new/avatars/dev.webp', tags: ['account'], mail: true },
    { name: 'Elena Torres', preview: 'Can we schedule a weekly report?', time: '2h', initials: 'ET', tone: 3, ai: true },
    { name: 'Lin Wei', preview: 'Rate limits for the public API', time: '3h', photo: '/new/avatars/lin.webp', tags: ['api'] },
    { name: 'Daniel Park', preview: 'Password reset email never arrived', time: '4h', initials: 'DP', tone: 0, tags: ['account'], mail: true },
  ];
  const row = (r: Row, selected: boolean, key: string, extra?: ReactNode) => <div className="hs-row" data-selected={selected} key={key}>
    {r.photo ? <Photo src={r.photo} size={34} /> : <Initials text={r.initials!} tone={r.tone!} />}
    <div className="hs-row-body">
      <div className="hs-row-top">{r.mail ? <Mail size={13} /> : <MessageCircle size={13} />}<strong>{r.name}</strong><small>{r.time}</small></div>
      <p><span>{r.preview}</span>{r.ai && <span className="hs-row-ai"><Bot size={10} /></span>}</p>
      {(r.chip || extra || r.tags) && <div className="hs-row-meta">{extra}{r.chip && <SLA label={r.chip} />}{r.tags?.map(t => <TagPill key={t} name={t} />)}</div>}
    </div>
  </div>;
  return <div className="hs-list">
    <div className="hs-head"><strong>Inbox</strong><Plus size={16} /><SlidersHorizontal size={15} /><Search size={16} /></div>
    <div className="hs-list-rows">
      {row(maya, viewing === 'maya', 'maya', sla && <SLA label={sla.label} tone={sla.tone} />)}
      {rows.map((r, i) => row(r, viewing === 'alex' && i === 0, r.name))}
    </div>
  </div>;
}

function Meta({ confidence, sources, channel = 'chat' }: { confidence: number; sources: number; channel?: 'chat' }) {
  return <div className="hs-meta"><span><CircleCheck size={12} />{confidence}% confident</span><span className="hs-source"><FileText size={11} />{sources} source{sources > 1 ? 's' : ''}<ChevronDown size={11} /></span>{channel === 'chat' && <MessageCircle size={12} />}<CheckCheck size={13} className="hs-read" /></div>;
}
function Customer({ text, time }: { text: string; time: string }) {
  return <In><div className="hs-msg hs-msg-in"><Photo src={MAYA} size={30} /><div className="hs-bubble">{text}<time>{time}</time></div></div></In>;
}
function AIReply({ text, time, meta }: { text: ReactNode; time: string; meta?: ReactNode }) {
  return <In><div className="hs-msg hs-msg-out"><div><div className="hs-bubble">{text}<time>{time}</time></div>{meta}</div><AIMark /></div></In>;
}
function Typing() { return <In><div className="hs-msg hs-msg-out"><div className="hs-bubble hs-typing"><i /><i /><i /></div><AIMark /></div></In>; }
function Event({ children }: { children: ReactNode }) { return <In><div className="hs-event">{children}</div></In>; }

function MayaThread({ stage, typingFor }: { stage: number; typingFor: number | null }) {
  return <>
    <div className="hs-date"><span />Today<span /></div>
    <Customer text="Hi! Does the CSV export include our custom fields?" time="09:10" />
    {typingFor === 1 && <Typing />}
    {stage >= 1 && <AIReply time="09:10" text="Yes. Custom fields are exported as extra columns after the standard ones, in the order they appear under Settings → Fields." meta={<Meta confidence={98} sources={1} />} />}
    {stage >= 2 && <Customer text="Thanks! When we open it in Excel, names with accents show up as garbled characters." time="09:11" />}
    {stage >= 3 && <AIReply time="09:11" text="Let me check how Excel reads CSV files. One moment." />}
    {stage >= 3 && <Event><span className="hs-agent-run" data-done={stage >= 4}>{stage >= 4 ? <CircleCheck size={13} /> : <span className="hs-spinner" />}<Globe size={13} />{stage >= 4 ? 'Research agent checked 2 pages on support.microsoft.com' : 'Research agent is reading support.microsoft.com'}</span></Event>}
    {typingFor === 4 && <Typing />}
    {stage >= 4 && <AIReply time="09:12" text={<>Your file is UTF-8, but Excel opens CSV files with your system’s legacy encoding unless told otherwise, which garbles accented characters. Open it with <b>Data → From Text/CSV</b> and choose <b>65001: Unicode (UTF-8)</b>, and the names will display correctly.</>} meta={<Meta confidence={94} sources={2} />} />}
    {stage >= 5 && <Customer text="That worked, thank you! Last thing: the export stops at 10,000 rows. We need all 18,400 for our monthly report." time="09:14" />}
    {stage >= 5 && <Event><Tag size={13} />Classified as a bug · Tagged <TagPill name="bug" /> <TagPill name="exports" /> · Priority set to High</Event>}
    {typingFor === 6 && <Typing />}
    {stage >= 6 && <>
      <AIReply time="09:15" text="I checked the export guide and your recent exports. Every one stopped at exactly 10,000 rows, which isn’t expected, so I’m bringing in our technical team." meta={<Meta confidence={96} sources={2} />} />
      <In><div className="hs-escalated"><AIMark size={20} />AI escalated to a human</div></In>
      <In><section className="hs-handoff">
        <header><AIMark size={20} /><strong>AI handoff</strong><span>Team only</span><time>9:15 AM</time></header>
        <p>Northstar Labs’ CSV exports stop at 10,000 of 18,400 contacts. They need the full list for a monthly report.</p>
        <dl>
          <div><dt>Reason</dt><dd>needs team investigation</dd></div>
          <div><dt>AI checked</dt><dd><span>Help articles:</span> “Export your contacts” sets no row limit.<br /><span>Export logs:</span> the last 3 exports each ended after 10,000 rows.</dd></div>
          <div><dt>Open question</dt><dd>Does every export over 10,000 rows stop at the first page?</dd></div>
        </dl>
      </section></In>
      <Event><Photo src={SAM} size={18} />Assigned to Sam · Technical Support</Event>
      <Event><CircleCheck size={13} className="hs-green" />Task created · <b>EXP-142</b> Complete exports beyond 10,000 rows</Event>
    </>}
    {stage >= 9 && stage < 12 && <Event><span className="hs-sla-line"><Clock3 size={13} />SLA due soon: next response target in 4 minutes (Urgent and high priority).</span></Event>}
    {stage >= 12 && <In><div className="hs-msg hs-msg-out"><div><div className="hs-bubble">{REPLY}<time>09:29</time></div><div className="hs-meta"><MessageCircle size={12} /><CheckCheck size={13} className="hs-read" /></div></div><Photo src={SAM} size={30} /></div></In>}
  </>;
}
function AlexThread() {
  return <>
    <div className="hs-date"><span />Today<span /></div>
    <In><div className="hs-msg hs-msg-in"><Initials text="AM" tone={2} size={30} /><div className="hs-bubble">Hi, could you resend last month’s invoice? Our accounts team can’t find it.<time>09:19</time></div></div></In>
    <In><div className="hs-msg hs-msg-in hs-mail-note"><span /><small><Mail size={11} />Received via email</small></div></In>
  </>;
}

function Details({ stage, viewing }: { stage: number; viewing: 'maya' | 'alex' }) {
  const sla = slaState(stage);
  if (viewing === 'alex') return <aside className="hs-details"><div className="hs-head"><strong>Details</strong></div><div className="hs-contact"><Initials text="AM" tone={2} size={46} /><strong>Alex Morgan</strong><span><Mail size={12} />alex@harborlane.example</span><em>Customer <ChevronDown size={11} /></em></div></aside>;
  return <aside className="hs-details">
    <div className="hs-head"><strong>Details</strong></div>
    <div className="hs-contact"><Photo src={MAYA} size={46} /><strong>Maya Chen</strong><span><Mail size={12} />maya@northstarlabs.example</span><em>Customer <ChevronDown size={11} /></em><a>View CRM Contact <ArrowUpRight size={11} /></a></div>
    <div className="hs-field"><b>Assignee</b>{stage >= 6 ? <span className="hs-in-inline"><Photo src={SAM} size={20} />Sam Rivera<ChevronDown size={12} /></span> : <span><AIMark size={20} />AI handling<ChevronDown size={12} /></span>}</div>
    <div className="hs-field"><b>Priority</b>{stage >= 5 ? <span className="hs-pill hs-in-inline"><BarChart3 size={12} className="hs-high" />High<ChevronsUpDown size={12} /></span> : <span className="hs-pill"><span className="hs-dim">No priority</span><ChevronsUpDown size={12} /></span>}</div>
    <div className="hs-field hs-sla-field"><div><b>SLA</b>{sla && <SLA label={sla.label} tone={sla.tone} />}</div>
      {stage >= 6 ? <ul><li>Urgent and high priority</li><li>First response: met</li><li className={sla?.tone === 'soon' ? 'hs-amber' : ''}>Next response: {stage >= 12 ? 'met' : stage >= 8 ? 'due in 4m' : stage === 7 ? 'due in 9m' : 'due in 15m'}</li></ul> : <ul><li>No open targets</li></ul>}
    </div>
    <div className="hs-field hs-tags-field"><div><b>Tags</b><Settings size={13} /></div><div>{stage >= 5 && <span className="hs-in-inline"><TagPill name="bug" /><TagPill name="exports" /></span>}<span className="hs-add"><Tag size={11} />Add</span></div></div>
    <div className="hs-field"><b>Company details</b><ChevronDown size={13} /></div>
    <dl className="hs-company"><div><dt>Company</dt><dd className="hs-link">Northstar Labs <ArrowUpRight size={11} /></dd></div><div><dt>Domain</dt><dd>northstarlabs.example</dd></div><div><dt>SLA tier</dt><dd>Standard</dd></div></dl>
    <div className="hs-field"><b>Tasks</b>{stage >= 6 ? <span className="hs-count">1</span> : <Plus size={13} />}</div>
    {stage >= 6 && <div className="hs-task hs-in-inline"><CircleCheck size={13} /><span><b>EXP-142</b>Complete exports beyond 10,000 rows</span></div>}
  </aside>;
}

function AskAgent({ stage }: { stage: number }) {
  return <aside className="hs-ask" data-stage={stage}>
    <header><Sparkles size={15} /><strong>Ask Agent</strong><span>Maya Chen · #142</span><CircleX size={15} /></header>
    <div className="hs-ask-body">
      <div className="hs-ask-q"><p>Why does Maya’s export stop at 10,000 rows, and what can I tell her?</p><Photo src={SAM} size={24} /></div>
      <div className="hs-ask-a">
        <div className="hs-ask-who"><AIMark size={22} /><strong>Helpin AI</strong><span>Read the conversation, logs and EXP-142</span></div>
        <h4>Why it happens</h4>
        <p>The export fetches contacts in pages of 10,000. For Northstar the second page is never requested, so 8,400 contacts are left out. It affects every export over 10,000 rows.</p>
        <div className="hs-evidence"><span><Code2 size={13} /><b>Export logs</b>3 exports ended after page 1 of 2</span><span><CircleCheck size={13} /><b>EXP-142</b>Pagination bug, fix not released</span><span><MessageCircle size={13} /><b>This conversation</b>Echo confirmed custom fields and encoding</span></div>
        <h4>What you can tell her</h4>
        <p>Exports under 10,000 rows are complete. For this month’s report, run two exports split by Created date. Promise an update in this conversation, not a release date.</p>
        <div className="hs-ask-actions"><span className="hs-insert">Insert as draft</span><span>Copy</span></div>
      </div>
    </div>
    <footer><span>Ask a follow-up…</span><Paperclip size={14} /></footer>
  </aside>;
}

function Toast() {
  return <div className="hs-toast"><span className="hs-toast-icon"><Clock3 size={15} /></span><div><strong>SLA due soon in #142</strong><p>The next response target of Urgent and high priority is due in 4m.</p><span className="hs-toast-open">Open conversation</span></div><MousePointer2 className="hs-cursor" size={20} /></div>;
}

export function SupportStory() {
  const frame = useRef<HTMLDivElement>(null);
  const thread = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(0);
  const [mobile, setMobile] = useState(false);
  const [paused, setPaused] = useState(false);
  const { container, phase: stage, playing } = useWorkflowPlayback({ beats: BEATS, duration: DURATION, paused });
  const active = playing && !paused;

  useEffect(() => {
    const element = frame.current;
    if (!element) return;
    const observer = new ResizeObserver(([entry]) => {
      const width = entry.contentRect.width;
      const narrow = width < 720;
      setMobile(narrow);
      setScale(width / (narrow ? MOBILE_W : W));
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const viewing = stage === 7 || stage === 8 ? 'alex' : 'maya';
  // AI replies are preceded by a short typing indicator in the first part of their beat.
  const [typingFor, setTypingFor] = useState<number | null>(null);
  useEffect(() => {
    setTypingFor(null);
    if (!active || ![1, 4, 6].includes(stage)) return;
    setTypingFor(stage);
    const timer = setTimeout(() => setTypingFor(null), 900);
    return () => clearTimeout(timer);
  }, [stage, active]);
  const shown = typingFor === stage ? stage - 1 : stage;

  useEffect(() => {
    const element = thread.current;
    if (!element) return;
    const id = requestAnimationFrame(() => element.scrollTo({ top: element.scrollHeight, behavior: active && stage > 0 && viewing === 'maya' && stage !== 9 ? 'smooth' : 'auto' }));
    return () => cancelAnimationFrame(id);
  }, [shown, typingFor, viewing, active, stage]);

  const sla = slaState(stage);
  const draft = stage === 11;
  const height = (mobile ? MOBILE_H : H) * scale;
  return <div ref={container} className="hs-root" data-playing={active}>
    <div ref={frame} className="hs-frame" style={{ height: scale ? height : undefined }} aria-label="Animated Helpin support inbox: the AI agent answers two questions, one with help from a research agent, classifies a bug report, hands it to a teammate with an SLA timer, and the teammate uses Ask Agent before replying.">
      {scale > 0 && <div className="hs-canvas" data-mobile={mobile} data-stage={stage} style={{ width: mobile ? MOBILE_W : W, height: mobile ? MOBILE_H : H, transform: `scale(${scale})` }} aria-hidden="true">
        {!mobile && <Navigation />}
        {!mobile && <ConversationList stage={stage} viewing={viewing} />}
        <div className="hs-thread">
          <div className="hs-head hs-thread-head">
            <strong>{viewing === 'maya' ? <>#142 - CSV export missing contacts</> : <>#139 - Invoice for last month</>}</strong>
            {viewing === 'maya' && sla && <SLA label={sla.label} tone={sla.tone} />}
            {!mobile && <span className="hs-btn"><CircleCheck size={13} />Create Task</span>}
            {!mobile && <span className="hs-icon-btn"><Clock3 size={14} /></span>}
            <span className="hs-resolve"><CircleCheck size={13} />Resolve</span>
            <MoreHorizontal size={16} className="hs-more" />
          </div>
          <div className="hs-messages" ref={thread}>{viewing === 'maya' ? <MayaThread stage={shown} typingFor={typingFor} /> : <AlexThread />}</div>
          <div className="hs-composer">
            <div className="hs-tabs"><span data-active="true">Reply</span><span>Note</span><span>Shortcuts</span><span className="hs-dim"><Sparkles size={12} />AI Tools<ChevronDown size={11} /></span><span className="hs-ask-btn" data-open={stage === 10}><Sparkles size={13} />Ask Agent</span></div>
            <div className="hs-input">{draft ? <span className="hs-typed">{REPLY}</span> : <span className="hs-placeholder">Write a reply… (@ to mention)</span>}</div>
            <div className="hs-tools"><Smile size={15} /><Paperclip size={15} /><i /><Bold size={14} /><Italic size={14} /><Underline size={14} /><Link2 size={14} /><i /><List size={14} /><ListOrdered size={14} /><Quote size={14} /><AtSign size={14} /><span className="hs-send-wrap"><kbd>Ctrl ↵</kbd><span className="hs-channel"><MessageCircle size={13} /><ChevronDown size={11} /></span><span className="hs-send" data-ready={draft}>Send</span></span></div>
          </div>
        </div>
        {!mobile && <Details stage={stage} viewing={viewing} />}
        {stage === 8 && <Toast />}
        {stage === 10 && <AskAgent stage={stage} />}
      </div>}
    </div>
    <button type="button" className="hs-playback" onClick={() => setPaused(value => !value)} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}{paused ? 'Play' : 'Pause'}</button>
    <span className="hs-sr-only" role="status">{['A customer asks about custom fields.', 'The AI agent answers from the docs.', 'She asks about garbled characters in Excel.', 'The AI agent asks her to hold and starts a research agent.', 'It answers with what the research agent found.', 'She reports a bug; Helpin tags it and sets the priority.', 'The AI agent hands over to Sam with a summary and an SLA starts.', 'Sam works another conversation.', 'An SLA reminder appears and Sam opens the conversation.', 'The conversation is back in view.', 'Sam asks Ask Agent why the export stops.', 'Sam drafts a detailed reply.', 'Sam sends the reply and the SLA is met.'][stage]}</span>
  </div>;
}
