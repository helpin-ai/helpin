'use client';

import { StreamingText } from '../../_components/StreamingText';

import '../../_components/product-previews/preview-navigation.css';

import { useEffect, useId, useRef, useState } from 'react';
import { ArrowUpRight, Bold, BookOpen, Bot, Building2, Check, CheckCheck, ChevronDown, ChevronRight, CircleCheck, ClipboardList, Clock3, Code2, Headphones, Inbox, LayoutGrid, Link2, List, Mail, MessageSquare, Minus, MoreHorizontal, Paperclip, Pause, Play, Plus, Search, Send, Settings, SlidersHorizontal, Smile, Sparkles, UserRound, X } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-workspace.css';

type Variant = 'inbox' | 'agent';
const DRAFT = 'Thanks, Maya. We’ve confirmed the full export is still stopping early. Sam has your earlier report and the latest findings. We’ll keep you updated in this conversation.';
const LABELS = {
  inbox: ['A customer question arrives', 'Helpin AI checks connected logs', 'Findings handed to Sam in Technical Support', 'EXP-142 links the issue to the work', 'Helpin AI prepares a reply', 'Your team reviews the reply before sending'],
  agent: ['Start with the customer’s question', 'Ask Agent reviews the context', 'Existing EXP-142 · Conversation attached', 'Coding agent prepares the change and tests', 'Ready for Sam’s review'],
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
function CustomerDetails({ phase }: { phase: number }) {
  return <aside className="swi-details" aria-hidden="true"><div className="swi-panel-head"><strong>Details</strong><ChevronRight size={13} /></div><div className="swi-contact"><Avatar person="maya" size={37} /><strong>Maya Chen</strong><span>maya@northstar.example</span><small><ArrowUpRight size={11} />View CRM contact</small></div><div className="swi-detail-group"><label>CONVERSATION ROUTING <ChevronDown size={10} /></label><div className="swi-detail-field"><Headphones size={12} /><span>{phase >= 1 ? 'Technical Support' : 'Shared inbox'}</span><ChevronDown size={10} /></div><div className="swi-detail-field"><Avatar person="sam" size={16} /><span>{phase >= 1 ? 'Sam Rivera' : 'Unassigned'}</span><ChevronDown size={10} /></div></div><div className="swi-detail-group"><label>TAGS <span>{phase >= 1 ? 2 : 0}</span></label><div className="swi-detail-tags" data-visible={phase >= 1}><Tags /><span><Plus size={10} />Add</span></div></div><div className="swi-detail-group"><label>CONTACT DETAILS <ChevronDown size={10} /></label><p><span>Lifecycle</span><em>Customer</em></p><p><span>Lead status</span><span>Active</span></p><p><span>Channel</span><span><Mail size={10} />Email</span></p></div><div className="swi-detail-group"><label>COMPANY DETAILS <ChevronDown size={10} /></label><div className="swi-company"><Building2 size={13} /><span><strong>Northstar Labs</strong><small>Enterprise · Customer</small></span></div></div><div className="swi-detail-group"><label>TASKS <span>{phase >= 2 ? 1 : 0}</span></label><div className="swi-linked-task" data-visible={phase >= 2}><ClipboardList size={13} /><span><strong>EXP-142</strong>Complete exports beyond 10,000 rows<small>In progress</small></span><ChevronRight size={10} /></div></div></aside>;
}
function Conversation({ phase, investigating }: { phase: number; investigating: boolean }) {
  return <div className="swi-messages" role="img" aria-label="Maya at Northstar Labs reports that a filtered CSV export stops at 10,000 of 18,400 rows. Helpin AI checks connected logs and finds that pagination stops at 10,000 rows, then tags and hands the issue to Sam in Technical Support. Task EXP-142 keeps the findings attached."><div aria-hidden="true"><div className="swi-date"><span />Today<span /></div><div className="swi-message"><Avatar person="maya" /><div><header><strong>Maya Chen</strong><small>09:14</small></header><p>Hi team, our CSV export stops at 10,000 rows. We need all 18,400 for our monthly report. Can you help?</p><small className="swi-email-source"><Mail size={10} />Received by email</small></div></div><div className="swi-route-event" data-visible={investigating || phase >= 1}><Bot size={12} /><span>{phase >= 1 ? <>Helpin AI tagged <Tags /> and handed to <strong>Sam · Technical Support</strong></> : <>Helpin AI is checking connected logs…</>}</span></div><div className="swi-message swi-internal" data-visible={phase >= 1}><Mark /><div><header><strong>Helpin AI</strong><small>Investigation note</small></header><p>Maya previously tried a smaller report. The full export still stops at 10,000 rows. Connected logs point to a pagination issue, not the customer’s filters.</p><div className="swi-task-chip" data-visible={phase >= 2}><Link2 size={11} /><strong>EXP-142</strong><span>Complete exports beyond 10,000 rows</span></div></div></div><div className="swi-message swi-confirmation" data-visible={phase >= 2}><Avatar person="maya" /><div><header><strong>Maya Chen</strong><small>09:18</small></header><p>The smaller report worked, but we still need the full list.</p></div></div></div></div>;
}
function AskAgent({ phase }: { phase: number }) {
  return <div className="swi-agent" data-open={phase >= 1} role="img" aria-label="Ask Agent investigates Maya’s export using existing task EXP-142, then asks the coding agent to prepare a pagination fix and regression test. The proposed change is sent to Sam for review; it has not been merged or deployed."><div aria-hidden="true"><div className="swi-agent-head"><Mark /><div><strong>Ask Agent</strong><small>Support · CSV export missing contacts</small></div><Minus size={15} /><X size={15} /></div><div className="swi-agent-body"><div className="swi-agent-question"><Avatar person="sam" /><p>Investigate Maya’s export using EXP-142. Ask the coding agent to prepare a fix and send it to me for review.</p></div><div className="swi-agent-answer"><div className="swi-agent-name"><Mark /><strong>Ask Agent</strong></div><p>I’ll use the linked task and keep Maya’s report with the proposed change.</p><div className="swi-agent-step" data-visible={phase >= 1}><CircleCheck size={17} /><div><strong>Reviewed the request</strong><p>The full export stops at 10,000 contacts. Maya needs all 18,400.</p></div></div><div className="swi-agent-step" data-visible={phase >= 2}><CircleCheck size={17} /><div><strong>Used the existing task</strong><p>EXP-142 · Customer report and investigation attached.</p><span className="swi-agent-source"><Link2 size={11} />Maya’s conversation attached</span></div></div><div className="swi-agent-step swi-code-step" data-visible={phase >= 3}><img src="/new/agents/forge.svg" width={22} height={22} alt="" /><div><strong>{phase === 3 ? 'Coding agent is preparing the fix' : 'Prepared a proposed fix'}</strong><p>{phase === 3 ? 'Checking pagination and adding a regression test.' : 'Pagination change and regression test ready.'}</p><span className="swi-code-line"><Code2 size={12} />+ export all filtered rows</span></div></div><div className="swi-agent-step swi-review-step" data-visible={phase >= 4}><Avatar person="sam" size={23} /><div><strong>Sent to Sam for review</strong><p>Changes awaiting review. Not merged or deployed.</p></div></div></div></div><div className="swi-agent-input"><div><span><MessageSquare size={11} />CSV export missing contacts <X size={10} /></span><Plus size={12} />Add context</div><p>Ask a follow-up…</p><footer><Paperclip size={13} /><span>Ask Agent</span><Send size={14} /></footer></div></div></div>;
}

function HumanAssist() {
  return <aside className="swi-human-assist" aria-label="Ask Agent helps Sam prepare a customer reply">
    <div className="swi-agent-head"><Mark /><div><strong>Ask Agent</strong><small>Only visible to your team</small></div></div>
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
  const { container, playing, cycle } = useBentoPlayback(21000, !scrollStory);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(4);
  const [draftMode, setDraftMode] = useState<'auto' | 'open' | 'used' | 'closed'>('auto');
  const [announcement, setAnnouncement] = useState('');
  const draftId = useId();
  const draftTrigger = useRef<HTMLButtonElement>(null);
  const active = playing && !paused && !scrollStory;
  const step = active ? frame : variant === 'inbox' ? 5 : 4;
  const phase = variant === 'inbox' ? Math.max(0, step - 1) : step;
  const draftVisible = draftMode !== 'closed' && phase >= 3;
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timings = variant === 'agent' ? [1700, 4200, 7200, 11000] : [1400, 3600, 5800, 8200, 12000];
    const timers = timings.map((time, i) => setTimeout(() => setFrame(i + 1), time));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle, variant]);
  function showDraft() { setPaused(true); setDraftMode('open'); setAnnouncement('AI draft ready to review.'); }
  function togglePlayback() { setPaused(value => !value); setDraftMode('auto'); setAnnouncement(''); }
  return <div className="support-workspace" data-variant={variant} data-scroll-story={scrollStory} data-assisted={assisted} data-panel-only={panelOnly} data-phase={phase} data-step={step} data-playing={active} data-highlight={highlight ?? undefined} ref={container}>
    {!scrollStory && <div className="swi-playback"><span>{LABELS[variant][step]}</span><button type="button" onClick={togglePlayback} aria-label={`${paused ? 'Play' : 'Pause'} ${variant === 'agent' ? 'Ask Agent workflow' : 'inbox workflow'} animation`} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>}
    <div className="swi-canvas">
      {!panelOnly && (<div className="swi-shell"><InboxNavigation /><ConversationList phase={phase} /><div className="swi-thread"><div className="swi-panel-head" aria-hidden="true"><strong><span>#142</span> — CSV export missing contacts</strong><ClipboardList size={14} /><span className="swi-resolve"><Check size={11} />Resolve</span><MoreHorizontal size={14} /></div><div className="swi-mobile-context" aria-hidden="true"><span><Headphones size={11} />{phase >= 1 ? 'Technical Support' : 'Shared inbox'}</span><Tags /></div><Conversation phase={phase} investigating={step >= 1} /><div className="swi-composer"><div className="swi-composer-tabs"><span aria-hidden="true">Reply</span><span aria-hidden="true">Note</span><span className="swi-shortcuts" aria-hidden="true">Shortcuts</span>{variant === 'inbox' ? <button ref={draftTrigger} type="button" onClick={showDraft} aria-expanded={draftVisible} aria-controls={draftId}><Sparkles size={12} />AI Tools<ChevronDown size={10} /></button> : <span className="swi-ai-tools-label" aria-hidden="true"><Sparkles size={12} />AI Tools<ChevronDown size={10} /></span>}<span className="swi-ask-label" aria-hidden="true"><Sparkles size={12} />Ask Agent</span></div><div className="swi-draft" id={draftId}><div className="swi-draft-content" hidden={!draftVisible}><div className="swi-draft-title"><span><Mark />{draftMode === 'used' ? 'Reply draft' : 'Ready for your review'}</span>{variant === 'inbox' && <button type="button" aria-label="Close AI draft" onClick={() => { setPaused(true); setDraftMode('closed'); setAnnouncement('Draft closed.'); draftTrigger.current?.focus({ preventScroll: true }); }}><X size={12} /></button>}</div><p aria-hidden="true"><StreamingText text={DRAFT} active={active && phase === 3 && variant === 'inbox'} duration={2400} /></p><span className="swi-sr-only">{DRAFT}</span></div><span className="swi-draft-placeholder" hidden={draftVisible}>Write a reply…</span></div><div className="swi-composer-bottom"><span aria-hidden="true"><Smile size={13} /><Paperclip size={13} /><Bold size={13} /><Link2 size={13} /></span>{draftVisible && variant === 'inbox' ? <button type="button" disabled={draftMode === 'used'} onClick={() => { setPaused(true); setDraftMode('used'); setAnnouncement('Draft added to the demo reply. Not sent.'); }}>{draftMode === 'used' ? <><Check size={11} />Draft added</> : 'Use draft'}</button> : <span className="swi-send" aria-hidden="true">Send <ChevronDown size={10} /></span>}</div></div></div><CustomerDetails phase={phase} /></div>)}
      {assisted && <HumanAssist />}
      {variant === 'agent' && <AskAgent phase={panelOnly ? Math.max(1, phase) : phase} />}
    </div><span className="swi-sr-only" role="status">{announcement}</span>
  </div>;
}
