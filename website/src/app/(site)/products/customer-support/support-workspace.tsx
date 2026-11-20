'use client';

import { StreamingText } from '../../_components/StreamingText';

import '../../_components/product-previews/preview-navigation.css';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ArrowUpRight, Bold, BookOpen, Bot, Building2, Check, CheckCheck, ChevronDown, ChevronRight, CircleCheck, ClipboardList, Clock3, Code2, Flag, Headphones, Inbox, LayoutGrid, Link2, List, Mail, MessageSquare, Minus, MoreHorizontal, MousePointer2, Paperclip, Pause, Play, Plus, Search, Send, Settings, SlidersHorizontal, Smile, Sparkles, Tag, UserRound, X } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowSources } from '../../_components/WorkflowParts';
import './support-inbox-showcase.css';
import './support-workspace.css';

type Variant = 'inbox' | 'agent';
const DRAFT = 'Maya, I found the cause: exports over 10,000 contacts stop after the first page, so 8,400 of yours are missing. For this month’s report, run two exports split by Created date and both files will be complete. Your report is attached to the fix (EXP-142), and I’ll update you here when it’s released.';
// Conversation stages: 0–4 Echo answers two questions (one through a research
// agent), 5–6 triage and handoff, 7 the SLA target opens, 8 its warning,
// 9 Sam asks Helpin AI, 10 Sam's reply is sent.
const BEATS = { inbox: [0, 1100, 3400, 4800, 7600, 10600, 12200, 16800, 18800, 21800, 26800], agent: [0, 1700, 4200, 7200, 11000] };

function Avatar({ person, size = 26 }: { person: 'sam' | 'maya'; size?: number }) {
  return <img className="swi-avatar" src={`/new/avatars/${person}.webp`} width={size} height={size} alt="" />;
}
function Mark() { return <span className="swi-ai-mark"><img src="/brand/helpin-icon-white.svg" width={16} height={16} alt="" /></span>; }
function Tags() { return <span className="swi-tags"><span>Bug</span><span>Exports</span></span>; }
// The next-response target starts with Maya's 09:18 message and is due at 09:33.
function SLAChip({ stage }: { stage: number }) {
  if (stage < 7) return null;
  return <span className="swi-sla" data-soon={stage === 8 || stage === 9}><Clock3 size={10} />{stage >= 10 ? 'Replied · SLA met' : stage >= 8 ? 'Reply due in 5m' : 'Reply due 09:33'}</span>;
}

// Presentational counterparts of SupportInboxLayout, ConversationRow,
// MessageThread, ReplyComposer and ConversationDetailSidebar. No app providers,
// customer API calls or visitor-widget session are used by this scripted scene.
function InboxNavigation() {
  return <div className="swi-navigation preview-sidebar" aria-hidden="true"><div className="swi-workspace"><span>O</span><strong>OrbitDesk</strong><ChevronDown size={12} /></div><div className="swi-nav-body"><div className="swi-app-rail">{[[LayoutGrid, 'Projects'], [MessageSquare, 'Support'], [BookOpen, 'Docs'], [Building2, 'CRM'], [Settings, 'Settings']].map(([Icon, title]) => { const I = Icon as typeof Inbox; return <span key={String(title)} data-current={title === 'Support'}><I size={16} /><small>{String(title)}</small></span>; })}<Avatar person="sam" /></div><div className="swi-queues">{[[Inbox, 'Inbox', '12'], [UserRound, 'Mine', '3'], [Clock3, 'Waiting', '5'], [CircleCheck, 'Resolved', ''], [X, 'Spam', '']].map(([Icon, title, count]) => { const I = Icon as typeof Inbox; return <div key={String(title)} data-current={title === 'Inbox'}><I size={13} /><span>{String(title)}</span><small>{String(count)}</small></div>; })}<p>HELPIN AI</p><div><Bot size={13} /><span>AI Handling</span><small>8</small></div><div><CheckCheck size={13} /><span>AI Resolved</span><small>24</small></div><p>TEAM INBOXES <Plus size={10} /></p>{['Technical Support', 'Billing', 'Feature Requests'].map((name, i) => <div key={name} className={i === 0 ? 'swi-route-target' : ''}><Inbox size={13} /><span>{name}</span><small>{[6, 3, 4][i]}</small></div>)}<p>SAVED VIEWS</p><div><List size={13} /><span>Needs engineering</span><small>4</small></div></div></div><div className="swi-search"><Search size={12} />Search OrbitDesk <small>⌘ K</small></div></div>;
}
function ConversationList({ stage }: { stage: number }) {
  const rows = [
    ['Maya Chen', 'CSV export missing contacts', '2m', 'M', 'chat'],
    ['Alex Morgan', 'Update invoice details', '14m', 'A', 'email', 'Billing'],
    ['Priya Shah', 'Invite our new teammates', '28m', 'P', 'chat'],
    ['Noah Williams', 'Webhook delivery retries', '1h', 'N', 'chat', 'API'],
    ['Elena Torres', 'Schedule a weekly report', '2h', 'E', 'email'],
    ['Daniel Park', 'API rate-limit question', '3h', 'D', 'chat', 'API'],
    ['Mia Brooks', 'Password reset email', '4h', 'M', 'email'],
  ];
  return <div className="swi-list" aria-hidden="true"><div className="swi-panel-head"><strong>Inbox</strong><Plus size={14} /><SlidersHorizontal size={13} /><Search size={14} /></div><div className="swi-list-filter"><span>Open <ChevronDown size={10} /></span><span>Newest <ChevronDown size={10} /></span></div>{rows.map(([name, subject, time, initial, channel, tag], index) => <div className="swi-conversation" key={name} data-selected={index === 0}>{index === 0 ? <Avatar person="maya" /> : <span className="swi-initial" data-color={index % 4}>{initial}</span>}<div><div className="swi-sender">{channel === 'email' ? <Mail size={10} /> : <MessageSquare size={10} />}<strong>{name}</strong><small>{time}</small></div><p>{subject}</p>{index === 0 && <span className="swi-row-meta" data-visible={stage >= 5}><SLAChip stage={stage} /><Tags /></span>}{tag && <span className="swi-tags swi-tag-plain"><span>{tag}</span></span>}</div></div>)}</div>;
}
function CustomerDetails({ stage, createdTask }: { stage: number; createdTask: boolean }) {
  const routed = stage >= 5;
  return <aside className="swi-details" aria-hidden="true"><div className="swi-panel-head"><strong>Details</strong><ChevronRight size={13} /></div><div className="swi-contact"><Avatar person="maya" size={37} /><strong>Maya Chen</strong><span>maya@northstar.example</span><small><ArrowUpRight size={11} />View CRM contact</small></div><div className="swi-detail-group"><label>CONVERSATION ROUTING <ChevronDown size={10} /></label><div className="swi-detail-field"><Headphones size={12} /><span>{routed ? 'Technical Support' : 'Shared inbox'}</span><ChevronDown size={10} /></div><div className="swi-detail-field"><Avatar person="sam" size={16} /><span>{stage >= 6 ? 'Sam Rivera' : 'Echo · AI handling'}</span><ChevronDown size={10} /></div><div className="swi-detail-field" data-urgent={routed}><Flag size={12} /><span>{routed ? 'Urgent' : 'No priority'}</span><ChevronDown size={10} /></div></div><div className="swi-detail-group"><label>TAGS <span>{routed ? 2 : 0}</span></label><div className="swi-detail-tags" data-visible={routed}><Tags /><span><Plus size={10} />Add</span></div></div><div className="swi-detail-group"><label>SLA · URGENT <SLAChip stage={stage} /></label><p><span>Next response</span><span>{stage >= 10 ? 'Met' : stage >= 8 ? 'Due in 5m' : stage >= 7 ? 'Due 09:33' : 'No open target'}</span></p></div><div className="swi-detail-group"><label>COMPANY DETAILS <ChevronDown size={10} /></label><div className="swi-company"><Building2 size={13} /><span><strong>Northstar Labs</strong><small>Enterprise · Customer</small></span></div></div><div className="swi-detail-group"><label>TASKS <span>{stage >= 6 ? 1 : 0}</span></label><div className="swi-linked-task" data-visible={stage >= 6}><ClipboardList size={13} /><span><strong>EXP-142</strong>Complete exports beyond 10,000 rows<small>{createdTask ? 'To do' : 'In progress'}</small></span><ChevronRight size={10} /></div></div></aside>;
}
function Conversation({ stage, active, createdTask, review, sentReply, sentLabel }: { stage: number; active: boolean; createdTask: boolean; review?: ReactNode; sentReply: string; sentLabel: string }) {
  const messages = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // Only the transcript follows new messages; scrolling the page stays natural.
    const element = messages.current;
    if (element) element.scrollTo({ top: stage === 0 ? 0 : element.scrollHeight, behavior: active && stage > 0 ? 'smooth' : 'auto' });
  }, [stage, review, sentReply, active]);
  const later = (ms: number) => ({ animationDelay: `${ms}ms` });
  const maya = (time: string, text: string, extra = '') => <div className={`swi-message swi-incoming ${extra}`}><Avatar person="maya" /><div><header><strong>Maya Chen</strong><small>{time}</small></header><p>{text}</p></div></div>;
  const echo = (time: string, text: string, on: number, meta = '') => <div className="swi-message swi-outgoing"><img className="swi-avatar" src="/new/agents/echo.svg" width={26} height={26} alt="" /><div><header><strong>Echo</strong><small>Support agent · {time}</small></header><p><StreamingText text={text} active={active && stage === on} duration={1300} /></p>{meta && <small className="swi-email-source"><CheckCheck size={11} />{meta}</small>}</div></div>;
  return <div className="swi-messages" ref={messages} role="region" aria-label="Example support conversation with Maya">
    <div className="swi-date"><span />Today<span /></div>
    {maya('09:10', 'Hi! Does the CSV export include our custom fields?')}
    {stage >= 1 && echo('09:10', 'Yes. Custom fields are exported as extra columns, after the standard ones.', 1, 'Answered from your docs · 98% confident · 1 source')}
    {stage >= 2 && maya('09:11', 'Thanks. The file shows broken characters when we open it in Excel, though.')}
    {stage >= 3 && <>
      {echo('09:11', 'Let me check how Excel reads this file. One moment.', 3)}
      <div className="swi-route-event swi-checking" data-done={stage >= 4}>{stage >= 4 ? <CircleCheck size={12} /> : <Bot size={12} />}<span>{stage >= 4 ? 'Research agent read 2 pages on Microsoft Support' : 'Research agent is reading Microsoft Support…'}</span></div>
    </>}
    {stage >= 4 && echo('09:12', 'Excel doesn’t detect UTF-8 by itself. Open the file with Data → From Text/CSV and choose UTF-8, and the characters will display correctly.', 4, '94% confident · 2 sources')}
    {stage >= 5 && <>
      {maya('09:14', 'That worked! One more thing: the export stops at 10,000 rows. We need all 18,400 for our monthly report.')}
      <div className="swi-route-event"><Tag size={12} /><span>Classified as a bug · Tagged <strong>Bug</strong> and <strong>Exports</strong></span></div>
      {stage === 5 && <div className="swi-route-event swi-checking"><Bot size={12} /><span>Echo is checking the export guide and connected logs…</span></div>}
    </>}
    {stage >= 6 && <>
      {echo('09:15', 'I checked the export guide and logs. Your export is stopping early. I’m bringing in Sam to investigate.', 6, '96% confident · 2 sources')}
      <div className="swi-escalated swi-later" style={later(1500)}><Mark />AI escalated to a human</div>
      <div className="swi-handoff swi-later" style={later(1900)}><header><Mark /><strong>AI handoff</strong><small>Team only</small></header><p>Northstar Labs can export only 10,000 of 18,400 contacts, and needs the full list for a monthly report.</p><dl><div><dt>Reason</dt><dd>Needs team investigation</dd></div><div><dt>AI checked</dt><dd>The export guide sets no row limit. Logs show the export stopping early.</dd></div></dl></div>
      <div className="swi-route-event swi-later" style={later(2300)}><Headphones size={12} /><span>Assigned to <strong>Sam · Technical Support</strong></span></div>
      <div className="swi-task-event swi-later" style={later(2800)} data-task-created={createdTask}><CircleCheck size={14} /><span><strong>{createdTask ? 'Task created' : 'Task linked'} · EXP-142</strong><small>Export issue · Customer report attached</small></span></div>
    </>}
    {stage >= 7 && maya('09:18', 'Thanks. Please let me know when the full export is ready.', 'swi-confirmation')}
    {createdTask && stage >= 8 && <div className="swi-sla-event"><Clock3 size={12} /><span>09:28 · SLA due soon: next response target in 5 minutes (Urgent priority).</span></div>}
    {review}
    {sentReply && <div className="swi-message swi-outgoing swi-sent-reply"><Avatar person="sam" /><div><header><strong>Sam Rivera</strong><small>09:29</small></header><p>{sentReply}</p><small className="swi-email-source"><CheckCheck size={11} />{sentLabel}</small></div></div>}
  </div>;
}

function AskAgent({ phase, active, paused, onTogglePlayback }: { phase: number; active: boolean; paused: boolean; onTogglePlayback: () => void }) {
  return <div className="swi-agent" data-open={phase >= 1} role="region" aria-label="Helpin AI investigates Maya’s export with the linked task and repository">
    <div><div className="swi-agent-head"><Mark /><div><strong>Helpin AI</strong><small>Support · Incomplete export</small></div><button type="button" className="swi-inline-playback" aria-label={`${paused ? 'Play' : 'Pause'} agent investigation animation`} aria-pressed={paused} onClick={onTogglePlayback}>{paused ? <Play size={12}/> : <Pause size={12}/>}</button></div>
      <div className="swi-agent-body"><div className="swi-agent-question"><Avatar person="sam" /><p>Investigate Maya’s export using EXP-142. Prepare a fix for my review.</p></div>
        <div className="swi-agent-answer"><div className="swi-agent-name"><Mark /><strong>Helpin AI</strong></div>
          {phase < 3 ? <><p>Reading the customer report and related work…</p><WorkflowSources sources={[{icon:MessageSquare,label:'Maya’s conversation',detail:'10,000 of 18,400 contacts exported'},{icon:Code2,label:'EXP-142 · Connected repository',detail:'Customer report, logs and export pagination'}]} phase={phase}/></> : <>
            <div className="swi-agent-step swi-code-step" data-visible="true"><img src="/new/agents/forge.svg" width={22} height={22} alt="" /><div><strong>Forge · Coding agent</strong><p>{phase === 3 ? 'Fixing pagination and testing the complete export…' : 'Pagination fixed · 18,400 rows verified'}</p><span className="swi-code-line"><Code2 size={12} />+ export all filtered rows</span></div></div>
            {phase >= 4 && <><p><StreamingText text="The fix and regression test are ready, with Maya’s report attached to the proposed change." active={active} duration={1600}/></p><div className="swi-agent-step swi-review-step" data-visible="true"><Avatar person="sam" size={23}/><div><strong>Ready for Sam’s review</strong><p>PR #728 · Awaiting approval</p></div></div></>}
            <span className="swi-agent-source"><Link2 size={11}/>Maya’s conversation · EXP-142 · Repository</span>
          </>}
        </div>
      </div>
      <div className="swi-agent-input"><div><span><MessageSquare size={11}/>EXP-142 <X size={10}/></span><Plus size={12}/>Add context</div><p>Ask a follow-up…</p><footer><Paperclip size={13}/><span>Helpin AI</span><Send size={14}/></footer></div>
    </div>
  </div>;
}

function HumanAssist() {
  return <aside className="swi-human-assist" aria-label="Helpin AI helps Sam prepare a customer reply">
    <div className="swi-agent-head"><Mark /><div><strong>Helpin AI</strong><small>Only visible to your team</small></div></div>
    <div className="swi-human-body">
      <div className="swi-agent-question"><Avatar person="sam" /><p>Why does Maya’s export stop at 10,000 rows, and what can I tell her?</p></div>
      <div className="swi-human-findings swi-later" style={{ animationDelay: '900ms' }}><span className="swi-human-label">WHY IT HAPPENS</span>
        <p>The export fetches contacts in pages of 10,000. For Northstar, page 2 is never requested, so 8,400 contacts are missing.</p>
        <div className="swi-human-source"><Code2 size={14} /><span>Connected logs<strong>Export job ended after page 1 of 2.</strong></span></div>
        <div className="swi-human-source"><ClipboardList size={14} /><span>Linked work · EXP-142<strong>Pagination bug. The fix is not released yet.</strong></span></div>
      </div>
      <div className="swi-human-draft swi-later" style={{ animationDelay: '2300ms' }}><span className="swi-human-label">WHAT YOU CAN TELL HER</span><p>Exports under 10,000 rows are complete. Suggest two exports split by Created date for this month’s report, and promise an update here, not a release date.</p></div>
      <div className="swi-human-review swi-later" style={{ animationDelay: '3300ms' }}><Avatar person="sam" size={22} /><span>Sam reviews the reply.<strong>Nothing is sent automatically.</strong></span></div>
    </div>
  </aside>;
}

export function SupportWorkspace({ variant = 'inbox', highlight = null, panelOnly = false, scrollStory = false, assisted = false }: { variant?: Variant; highlight?: number | null; panelOnly?: boolean; scrollStory?: boolean; assisted?: boolean }) {
  const [paused, setPaused] = useState(false);
  const { container, playing, cycle, phase: frame } = useWorkflowPlayback({ paused: paused || scrollStory, beats: BEATS[variant], duration: variant === 'agent' ? 21000 : 37000 });
  const [draftMode, setDraftMode] = useState<'auto' | 'open' | 'used' | 'closed'>('auto');
  const [announcement, setAnnouncement] = useState('');
  const [reply, setReply] = useState('');
  const [sentReply, setSentReply] = useState('');
  const replyInput = useRef<HTMLTextAreaElement>(null);
  const draftId = useId();
  const draftTrigger = useRef<HTMLButtonElement>(null);
  const active = playing && !paused && !scrollStory;
  const story = variant === 'inbox' && !scrollStory;
  // The agent dock opens on the handoff; the scroll story rests on the open SLA target.
  const step = variant === 'agent' ? (scrollStory ? 4 : frame) : scrollStory ? 7 : frame;
  const stage = variant === 'agent' ? (step === 0 ? 5 : 6) : step;
  // Phase drives the agent dock and the routing highlight.
  const phase = variant === 'agent' ? step : stage < 5 ? 0 : stage < 7 ? 1 : stage < 8 ? 2 : 3;
  const draftVisible = variant === 'inbox' && (draftMode === 'open' || (draftMode === 'auto' && scrollStory));
  useEffect(() => {
    const input = replyInput.current;
    if (!input) return;
    input.style.height = '48px';
    input.style.height = `${Math.min(96, Math.max(48, input.scrollHeight))}px`;
  }, [reply]);
  useEffect(() => {
    setReply('');
    setSentReply('');
    setDraftMode('auto');
  }, [cycle, variant]);
  function showDraft() { setPaused(true); setDraftMode('open'); setAnnouncement('AI draft ready to review.'); }
  function togglePlayback() { setPaused(value => !value); setDraftMode('auto'); setAnnouncement(''); }
  function useDraft() {
    setPaused(true); setDraftMode('used'); setReply(DRAFT);
    setAnnouncement('Draft added to the reply field. Not sent.');
    replyInput.current?.focus({ preventScroll: true });
  }
  function sendDemoReply() {
    if (!reply.trim()) return;
    setPaused(true); setDraftMode('closed'); setSentReply(reply.trim()); setReply('');
    setAnnouncement('Reply added to this demo conversation. No real message was sent.');
  }
  const draftReview = draftVisible ? <div className="swi-review-card" id={draftId}>
    <div className="swi-draft-title"><span><Mark />AI draft · Not sent</span><button type="button" aria-label="Close AI draft" onClick={() => { setPaused(true); setDraftMode('closed'); setAnnouncement('Draft closed.'); draftTrigger.current?.focus({ preventScroll: true }); }}><X size={12} /></button></div>
    <p>{DRAFT}</p>
    <footer><span>For Sam to review</span><button type="button" onClick={useDraft}>Use draft</button></footer>
  </div> : null;
  const autoSent = story && stage >= 10 && !sentReply;
  return <div className="support-workspace" data-variant={variant} data-scroll-story={scrollStory} data-assisted={assisted} data-panel-only={panelOnly} data-phase={phase} data-step={step} data-playing={active} data-highlight={highlight ?? undefined} ref={container}>

    <div className="swi-canvas">
      {!panelOnly && (<div className="swi-shell"><InboxNavigation /><ConversationList stage={stage} /><div className="swi-thread"><div className="swi-panel-head"><strong><span>#142</span> · CSV export missing contacts</strong><SLAChip stage={stage} /><ClipboardList size={14} /><span className="swi-resolve"><Check size={11} />Resolve</span><MoreHorizontal size={14} />{!scrollStory && variant === 'inbox' && <button className="swi-inline-playback" type="button" onClick={togglePlayback} aria-label={`${paused ? 'Play' : 'Pause'} inbox animation`} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}</button>}</div><div className="swi-mobile-context" aria-hidden="true"><span><Headphones size={11} />{stage >= 5 ? 'Technical Support' : 'Shared inbox'}</span><span data-visible={stage >= 5}><Tags /></span></div><Conversation stage={stage} active={active} createdTask={variant === 'inbox'} review={draftReview} sentReply={sentReply || (autoSent ? DRAFT : '')} sentLabel={sentReply ? 'Demo reply' : 'Sent'} />
          <div className="swi-composer">
            <div className="swi-composer-tabs"><span aria-hidden="true">Reply</span><span aria-hidden="true">Note</span><span className="swi-shortcuts" aria-hidden="true">Shortcuts</span>{variant === 'inbox' ? <button ref={draftTrigger} type="button" onClick={showDraft} aria-expanded={draftVisible} aria-controls={draftVisible ? draftId : undefined}><Sparkles size={12} />AI Tools<ChevronDown size={10} /></button> : <span className="swi-ai-tools-label" aria-hidden="true"><Sparkles size={12} />AI Tools<ChevronDown size={10} /></span>}<span className="swi-ask-label" aria-hidden="true" data-open={story && stage === 9}><Sparkles size={12} />Helpin AI</span></div>
            <textarea className="swi-reply-input" ref={replyInput} rows={2} aria-label="Write a reply (demo)" placeholder="Write a reply…" value={reply} onFocus={() => setPaused(true)} onChange={event => { setPaused(true); setReply(event.target.value); }} />
            <div className="swi-composer-bottom"><span aria-hidden="true"><Smile size={13} /><Paperclip size={13} /><Bold size={13} /><Link2 size={13} /></span><button type="button" disabled={!reply.trim()} onClick={sendDemoReply} aria-label="Send reply in demo">Send <ChevronDown size={10} /></button></div>
          </div></div><CustomerDetails stage={stage} createdTask={variant === 'inbox'} /></div>)}
      {story && stage === 8 && <div className="swi-toast" aria-hidden="true"><Clock3 size={15} /><div><strong>SLA due soon in #142</strong><p>The next response target of Urgent priority is due in 5m.</p><span>Open conversation</span></div><MousePointer2 className="swi-cursor" size={18} /></div>}
      {(assisted || (story && stage === 9)) && <HumanAssist />}
      {variant === 'agent' && <AskAgent phase={panelOnly ? Math.max(1, phase) : phase} active={active} paused={paused} onTogglePlayback={togglePlayback} />}
    </div><span className="swi-sr-only" role="status">{announcement}</span>
  </div>;
}
