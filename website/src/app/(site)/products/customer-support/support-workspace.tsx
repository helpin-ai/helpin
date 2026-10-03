'use client';

import { StreamingText } from '../../_components/StreamingText';

import '../../_components/product-previews/preview-navigation.css';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ArrowUpRight, Bold, BookOpen, Bot, Building2, Check, CheckCheck, ChevronDown, ChevronRight, CircleCheck, ClipboardList, Clock3, Code2, Headphones, Inbox, LayoutGrid, Link2, List, Mail, MessageSquare, Minus, MoreHorizontal, Paperclip, Pause, Play, Plus, Search, Send, Settings, SlidersHorizontal, Smile, Sparkles, UserRound, X } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowSources } from '../../_components/WorkflowParts';
import './support-workspace.css';

type Variant = 'inbox' | 'agent';
const DRAFT = 'The export issue is now tracked in EXP-142 with your report attached. We’ll update you here when the fix is ready.';
const LABELS = {
  inbox: ['A customer question arrives', 'Echo checks the export guide and connected logs', 'Echo replies and brings in Sam', 'Task created · EXP-142 · Customer report attached', 'Helpin AI drafts Sam’s follow-up', 'Sam reviews the draft before sending'],
  agent: ['Start with the customer’s question', 'Helpin AI reviews the context', 'Existing EXP-142 · Conversation attached', 'Coding agent prepares the change and tests', 'Ready for Sam’s review'],
};

function Avatar({ person, size = 26 }: { person: 'sam' | 'maya'; size?: number }) {
  return <img className="swi-avatar" src={`/new/avatars/${person}.webp`} width={size} height={size} alt="" />;
}
function Mark() { return <span className="swi-ai-mark"><img src="/brand/helpin-icon-white.svg" width={16} height={16} alt="" /></span>; }
function Tags() { return <span className="swi-tags"><span>Bug</span><span>Exports</span></span>; }

// Presentational counterparts of SupportInboxLayout, ConversationRow,
// MessageThread, ReplyComposer and ConversationDetailSidebar. No app providers,
// customer API calls or visitor-widget session are used by this scripted scene.
function InboxNavigation() {
  return <div className="swi-navigation preview-sidebar" aria-hidden="true"><div className="swi-workspace"><span>O</span><strong>OrbitDesk</strong><ChevronDown size={12} /></div><div className="swi-nav-body"><div className="swi-app-rail">{[[LayoutGrid, 'Projects'], [MessageSquare, 'Support'], [BookOpen, 'Docs'], [Building2, 'CRM'], [Settings, 'Settings']].map(([Icon, title]) => { const I = Icon as typeof Inbox; return <span key={String(title)} data-current={title === 'Support'}><I size={16} /><small>{String(title)}</small></span>; })}<Avatar person="sam" /></div><div className="swi-queues">{[[Inbox, 'Inbox', '12'], [UserRound, 'Mine', '3'], [Clock3, 'Waiting', '5'], [CircleCheck, 'Resolved', ''], [X, 'Spam', '']].map(([Icon, title, count]) => { const I = Icon as typeof Inbox; return <div key={String(title)} data-current={title === 'Inbox'}><I size={13} /><span>{String(title)}</span><small>{String(count)}</small></div>; })}<p>HELPIN AI</p><div><Bot size={13} /><span>AI Handling</span><small>8</small></div><div><CheckCheck size={13} /><span>AI Resolved</span><small>24</small></div><p>TEAM INBOXES <Plus size={10} /></p>{['Technical Support', 'Billing', 'Feature Requests'].map((name, i) => <div key={name} className={i === 0 ? 'swi-route-target' : ''}><Inbox size={13} /><span>{name}</span><small>{[6, 3, 4][i]}</small></div>)}<p>SAVED VIEWS</p><div><List size={13} /><span>Needs engineering</span><small>4</small></div></div></div><div className="swi-search"><Search size={12} />Search OrbitDesk <small>⌘ K</small></div></div>;
}
function ConversationList({ phase }: { phase: number }) {
  const rows = [
    ['Maya Chen', 'CSV export missing contacts', '2m', 'M', 'email'],
    ['Alex Morgan', 'Update invoice details', '14m', 'A', 'email'],
    ['Priya Shah', 'Invite our new teammates', '28m', 'P', 'chat'],
    ['Noah Williams', 'Webhook delivery retries', '1h', 'N', 'chat'],
    ['Elena Torres', 'Schedule a weekly report', '2h', 'E', 'email'],
    ['Daniel Park', 'API rate-limit question', '3h', 'D', 'chat'],
    ['Mia Brooks', 'Password reset email', '4h', 'M', 'email'],
  ];
  return <div className="swi-list" aria-hidden="true"><div className="swi-panel-head"><strong>Inbox</strong><Plus size={14} /><SlidersHorizontal size={13} /><Search size={14} /></div><div className="swi-list-filter"><span>Open <ChevronDown size={10} /></span><span>Newest <ChevronDown size={10} /></span></div>{rows.map(([name, subject, time, initial, channel], index) => <div className="swi-conversation" key={name} data-selected={index === 0}>{index === 0 ? <Avatar person="maya" /> : <span className="swi-initial" data-color={index % 4}>{initial}</span>}<div><div className="swi-sender">{channel === 'email' ? <Mail size={10} /> : <MessageSquare size={10} />}<strong>{name}</strong><small>{time}</small></div><p>{subject}</p>{index === 0 && <span data-visible={phase >= 1}><Tags /></span>}</div></div>)}</div>;
}
function CustomerDetails({ phase, createdTask }: { phase: number; createdTask: boolean }) {
  return <aside className="swi-details" aria-hidden="true"><div className="swi-panel-head"><strong>Details</strong><ChevronRight size={13} /></div><div className="swi-contact"><Avatar person="maya" size={37} /><strong>Maya Chen</strong><span>maya@northstar.example</span><small><ArrowUpRight size={11} />View CRM contact</small></div><div className="swi-detail-group"><label>CONVERSATION ROUTING <ChevronDown size={10} /></label><div className="swi-detail-field"><Headphones size={12} /><span>{phase >= 1 ? 'Technical Support' : 'Shared inbox'}</span><ChevronDown size={10} /></div><div className="swi-detail-field"><Avatar person="sam" size={16} /><span>{phase >= 1 ? 'Sam Rivera' : 'Unassigned'}</span><ChevronDown size={10} /></div></div><div className="swi-detail-group"><label>TAGS <span>{phase >= 1 ? 2 : 0}</span></label><div className="swi-detail-tags" data-visible={phase >= 1}><Tags /><span><Plus size={10} />Add</span></div></div><div className="swi-detail-group"><label>CONTACT DETAILS <ChevronDown size={10} /></label><p><span>Lifecycle</span><em>Customer</em></p><p><span>Lead status</span><span>Active</span></p><p><span>Channel</span><span><Mail size={10} />Email</span></p></div><div className="swi-detail-group"><label>COMPANY DETAILS <ChevronDown size={10} /></label><div className="swi-company"><Building2 size={13} /><span><strong>Northstar Labs</strong><small>Enterprise · Customer</small></span></div></div><div className="swi-detail-group"><label>TASKS <span>{phase >= 2 ? 1 : 0}</span></label><div className="swi-linked-task" data-visible={phase >= 2}><ClipboardList size={13} /><span><strong>EXP-142</strong>Complete exports beyond 10,000 rows<small>{createdTask ? 'To do' : 'In progress'}</small></span><ChevronRight size={10} /></div></div></aside>;
}
function Conversation({ phase, investigating, active, createdTask, review, sentReply }: { phase: number; investigating: boolean; active: boolean; createdTask: boolean; review?: ReactNode; sentReply: string }) {
  const messages = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // Only the transcript follows new messages; scrolling the page stays natural.
    if (messages.current) messages.current.scrollTop = phase === 0 && !investigating ? 0 : messages.current.scrollHeight;
  }, [phase, investigating, review, sentReply]);
  return <div className="swi-messages" ref={messages} role="region" aria-label="Example support conversation with Maya">
    <div className="swi-date"><span />Today<span /></div>
    <div className="swi-message swi-incoming"><Avatar person="maya" /><div><header><strong>Maya Chen</strong><small>09:14</small></header><p>Hi team, our CSV export stops at 10,000 rows. We need all 18,400 for our monthly report. Can you help?</p><small className="swi-email-source"><Mail size={10} />Received by email</small></div></div>
    {investigating && phase === 0 && <div className="swi-route-event swi-checking"><Bot size={12} /><span>Echo is checking the export guide and connected logs…</span></div>}
    {phase >= 1 && <>
      <div className="swi-message swi-outgoing"><img className="swi-avatar" src="/new/agents/echo.svg" width={26} height={26} alt="" /><div><header><strong>Echo</strong><small>Support agent · 09:15</small></header><p><StreamingText text="I checked the export guide and logs. Your export is stopping early. I’m bringing in Sam to investigate." active={active && phase === 1} duration={1500} /></p><small className="swi-email-source"><CheckCheck size={11} />Sent via email</small></div></div>
      <div className="swi-route-event"><Headphones size={12} /><span>Assigned to <strong>Sam · Technical Support</strong></span></div>
    </>}
    {phase >= 2 && <>
      <div className="swi-task-event" data-task-created={createdTask}><CircleCheck size={14} /><span><strong>{createdTask ? 'Task created' : 'Task linked'} · EXP-142</strong><small>Export issue · Customer report attached</small></span></div>
      <div className="swi-message swi-incoming swi-confirmation"><Avatar person="maya" /><div><header><strong>Maya Chen</strong><small>09:18</small></header><p>Thanks. Please let me know when the full export is ready.</p></div></div>
    </>}
    {review}
    {sentReply && <div className="swi-message swi-outgoing swi-sent-reply"><Avatar person="sam" /><div><header><strong>Sam Rivera</strong><small>09:19</small></header><p>{sentReply}</p><small className="swi-email-source"><CheckCheck size={11} />Demo reply</small></div></div>}
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
      <div className="swi-agent-question"><Avatar person="sam" /><p>What have we tried for Maya, and what should I tell her next?</p></div>
      <div className="swi-human-findings"><span className="swi-human-label">CONTEXT FOR YOUR REPLY</span>
        <p>The smaller report worked. The full export still stops at 10,000 of 18,400 contacts.</p>
        <div className="swi-human-source"><MessageSquare size={14} /><span>Earlier conversation<strong>Maya confirmed the smaller export.</strong></span></div>
        <div className="swi-human-source"><ClipboardList size={14} /><span>Linked work · EXP-142<strong>Pagination fix in progress.</strong></span></div>
      </div>
      <div className="swi-human-draft"><span className="swi-human-label">SUGGESTED NEXT STEP</span><p>Keep Maya updated in this conversation. Explain that the full export is still being investigated; don’t promise a release date.</p></div>
      <div className="swi-human-review"><Avatar person="sam" size={22} /><span>Sam reviews the reply.<strong>Nothing is sent automatically.</strong></span></div>
    </div>
  </aside>;
}

export function SupportWorkspace({ variant = 'inbox', highlight = null, panelOnly = false, scrollStory = false, assisted = false }: { variant?: Variant; highlight?: number | null; panelOnly?: boolean; scrollStory?: boolean; assisted?: boolean }) {
  const [paused, setPaused] = useState(false);
  const { container, playing, cycle, phase: frame } = useWorkflowPlayback({ paused: paused || scrollStory, beats: variant === 'agent' ? [0,1700,4200,7200,11000] : [0,1200,2800,4700,6700,9600], duration: 21000 });
  const [draftMode, setDraftMode] = useState<'auto' | 'open' | 'used' | 'closed'>('auto');
  const [announcement, setAnnouncement] = useState('');
  const [reply, setReply] = useState('');
  const [sentReply, setSentReply] = useState('');
  const replyInput = useRef<HTMLTextAreaElement>(null);
  const draftId = useId();
  const draftTrigger = useRef<HTMLButtonElement>(null);
  const active = playing && !paused && !scrollStory;
  const step = scrollStory ? (variant === 'inbox' ? 5 : 4) : frame;
  const phase = variant === 'inbox' ? Math.max(0, step - 1) : step;
  const draftVisible = variant === 'inbox' && (draftMode === 'auto' || draftMode === 'open') && phase >= 3;
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
    <p><StreamingText text={DRAFT} active={active && phase === 3} duration={2100} /></p>
    <footer><span>For Sam to review</span><button type="button" onClick={useDraft}>Use draft</button></footer>
  </div> : null;
  return <div className="support-workspace" data-variant={variant} data-scroll-story={scrollStory} data-assisted={assisted} data-panel-only={panelOnly} data-phase={phase} data-step={step} data-playing={active} data-highlight={highlight ?? undefined} ref={container}>

    <div className="swi-canvas">
      {!panelOnly && (<div className="swi-shell"><InboxNavigation /><ConversationList phase={phase} /><div className="swi-thread"><div className="swi-panel-head"><strong><span>#142</span> · CSV export missing contacts</strong><ClipboardList size={14} /><span className="swi-resolve"><Check size={11} />Resolve</span><MoreHorizontal size={14} />{!scrollStory && variant === 'inbox' && <button className="swi-inline-playback" type="button" onClick={togglePlayback} aria-label={`${paused ? 'Play' : 'Pause'} inbox animation`} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}</button>}</div><div className="swi-mobile-context" aria-hidden="true"><span><Headphones size={11} />{phase >= 1 ? 'Technical Support' : 'Shared inbox'}</span><Tags /></div><Conversation phase={phase} investigating={step >= 1} active={active} createdTask={variant === 'inbox'} review={draftReview} sentReply={sentReply} />
          <div className="swi-composer">
            <div className="swi-composer-tabs"><span aria-hidden="true">Reply</span><span aria-hidden="true">Note</span><span className="swi-shortcuts" aria-hidden="true">Shortcuts</span>{variant === 'inbox' ? <button ref={draftTrigger} type="button" onClick={showDraft} aria-expanded={draftVisible} aria-controls={draftVisible ? draftId : undefined}><Sparkles size={12} />AI Tools<ChevronDown size={10} /></button> : <span className="swi-ai-tools-label" aria-hidden="true"><Sparkles size={12} />AI Tools<ChevronDown size={10} /></span>}<span className="swi-ask-label" aria-hidden="true"><Sparkles size={12} />Helpin AI</span></div>
            <textarea className="swi-reply-input" ref={replyInput} rows={2} aria-label="Write a reply (demo)" placeholder="Write a reply…" value={reply} onFocus={() => setPaused(true)} onChange={event => { setPaused(true); setReply(event.target.value); }} />
            <div className="swi-composer-bottom"><span aria-hidden="true"><Smile size={13} /><Paperclip size={13} /><Bold size={13} /><Link2 size={13} /></span><button type="button" disabled={!reply.trim()} onClick={sendDemoReply} aria-label="Send reply in demo">Send <ChevronDown size={10} /></button></div>
          </div></div><CustomerDetails phase={phase} createdTask={variant === 'inbox'} /></div>)}
      {assisted && <HumanAssist />}
      {variant === 'agent' && <AskAgent phase={panelOnly ? Math.max(1, phase) : phase} active={active} paused={paused} onTogglePlayback={togglePlayback} />}
    </div><span className="swi-sr-only" role="status">{announcement}</span>
  </div>;
}
