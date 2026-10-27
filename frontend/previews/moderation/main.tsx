import { useCallback, useEffect, useRef, useState, Fragment } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { mountWidget, unmountWidget, type Message, type WidgetConfig } from '@helpin-ai/widget-core';
import { MessageBubble } from '@/components/support/MessageBubble';
import type { SupportMessage } from '@/lib/pmTypes';
import { Button } from '@/components/ui/button';
import { TooltipProvider, Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Inbox, Shield, EyeOff, Eye, MessageCircle, BookOpen, Bot, Search, SlidersHorizontal, ChevronDown, Check, User, RotateCcw, Send, Info, Image as ImageIcon, PanelRightClose, X } from 'lucide-react';
import '@helpin-ai/widget-core/styles';
import './style.css';

const now = new Date(Date.now() - 120_000).toISOString();
const initialMessages: Message[] = [
  { id: 'report', conversationId: 'demo', role: 'customer', content: 'Someone sent me an inappropriate image. Can you help me report it?', isInternal: false, createdAt: now },
  { id: 'held', conversationId: 'demo', role: 'system', content: '**Attachment held for review**\n\nYour message was sent. Our support team will review the attachment.', isInternal: false, createdAt: new Date(Date.parse(now) + 1000).toISOString() },
  { id: 'ack', conversationId: 'demo', role: 'agent', senderName: 'Helpin Support', content: 'We’ve sent your report to our support team.', isInternal: false, createdAt: new Date(Date.parse(now) + 2000).toISOString() },
];
const config: WidgetConfig = {
  workspaceId: 'demo', workspaceName: 'Helpin Support', visitorName: 'Alex',
  branding: { primaryColor: '#27272a', welcomeMessage: 'How can we help?', widgetPosition: 'bottom-right', showBranding: true, colorScheme: 'light' },
  features: { aiEnabled: false, aiFirst: false, showTalkToHuman: false, fileUploads: false, preChatForm: false, requirePhone: false, csatRating: false, forceIdentify: false, emailCaptureTiming: 'never' },
  availability: { isOnline: true, statusText: 'Online', replyTimeText: 'We typically reply in a few minutes' },
};
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function Widget({ messages, onSend }: { messages: Message[]; onSend: (text: string) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(true);
  useEffect(() => {
    if (!ref.current) return;
    mountWidget(ref.current, {
      config, messages, isOpen: open, showLauncher: true,
      onClose: () => setOpen(false), onLauncherClick: () => setOpen(value => !value),
      onSendMessage: onSend, initialView: 'conversation', connectionStatus: 'connected',
      activeConversation: { id: 'demo', subject: 'Report of inappropriate content', status: 'open', aiState: 'handoff', flowState: 'human_active' },
      activeTeammate: { userId: 'demo-agent', name: 'Support team', status: 'online' },
    });
  }, [messages, onSend, open]);
  useEffect(() => { const el = ref.current; return () => { if (el) unmountWidget(el); }; }, []);
  return <div className="widget-stage"><div className="site-placeholder"><span className="site-logo">acme<span>®</span></span><span>Help center</span></div><div className="site-copy"><small>HERE TO HELP</small><h2>A little help.<br/>A better day.</h2><p>Find answers or talk to our team.</p><div className="fake-search"><Search size={17}/> Search for answers</div></div><div ref={ref} className="widget-mount"/></div>;
}

function Attachment() {
  const [visible, setVisible] = useState(false);
  return <div className="sensitive-card">
    {visible ? <div className="sample-attachment"><ImageIcon size={32}/><strong>Sample attachment</strong><span>Safe placeholder for this preview</span></div> : <div className="hidden-attachment"><EyeOff size={23}/><strong>Sensitive attachment hidden</strong><span>Potentially explicit content</span></div>}
    <div className="attachment-footer"><span><Shield size={13}/> Not shared with AI</span><Button variant="outline" size="sm" onClick={() => setVisible(value => !value)} aria-expanded={visible}>{visible ? <EyeOff/> : <Eye/>}{visible ? 'Hide attachment' : 'Show attachment'}</Button></div>
  </div>;
}

function asSupportMessage(message: Message): SupportMessage {
  return { id: message.id, workspace_id: 'demo', conversation_id: 'demo', sender_type: message.role === 'agent' ? 'user' : 'customer',
    sender_display_name: message.role === 'customer' ? 'Alex Morgan' : message.senderName || 'You',
    content: message.content, is_internal: false, created_at: message.createdAt, updated_at: message.createdAt, via_channel: 'widget' };
}

function App() {
  const [view, setView] = useState<'both' | 'inbox' | 'widget'>('both');
  const [messages, setMessages] = useState(initialMessages);
  const [draft, setDraft] = useState('');
  const [assigned, setAssigned] = useState(false);
  const [resolved, setResolved] = useState(false);
  const [details, setDetails] = useState(false);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('Inbox');
  const threadRef = useRef<HTMLDivElement>(null);
  const nextMessageId = useRef(0);
  const send = useCallback((content: string, role: Message['role']) => {
    if (!content.trim()) return;
    const id = `demo-message-${++nextMessageId.current}`;
    setMessages(previous => [...previous, { id, conversationId: 'demo', role, content: content.trim(), senderName: role === 'agent' ? 'Sam · Support' : 'Alex Morgan', isInternal: false, createdAt: new Date().toISOString() }]);
    setResolved(false);
  }, []);
  const customerSend = useCallback((content: string) => send(content, 'customer'), [send]);
  useEffect(() => { threadRef.current?.scrollTo({ top: threadRef.current.scrollHeight, behavior: 'smooth' }); }, [messages]);
  const shown = 'Alex Morgan Report of inappropriate content'.toLowerCase().includes(query.toLowerCase()) && (filter !== 'Resolved' || resolved) && (filter !== 'Mine' || assigned);
  return <TooltipProvider><div className="preview-app">
    <header className="preview-toolbar"><div><span className="preview-dot"/><strong>Support safety</strong><span className="demo-tag">Interactive preview</span></div><div className="view-switch" aria-label="Preview view">{(['both','inbox','widget'] as const).map(value => <button key={value} aria-pressed={view === value} onClick={() => setView(value)}>{value === 'both' ? 'Side by side' : value === 'inbox' ? 'Support inbox' : 'Customer widget'}</button>)}</div><Button size="sm" variant="ghost" onClick={() => { setMessages([...initialMessages]); setDraft(''); setResolved(false); setAssigned(false); setQuery(''); setFilter('Inbox'); }}><RotateCcw/>Reset</Button></header>
    <div className="preview-note">Demo data only. Send a message on either side to see the conversation update.</div>
    <main className={`preview-panels view-${view}`}>
      {view !== 'widget' && <section className="inbox-panel" aria-label="Support inbox"><div className="panel-label"><Inbox size={14}/> Support inbox <span>Agent view</span></div><div className="app-shell">
        <aside className="app-rail"><div className="brand-mark">h</div><div className="rail-active"><MessageCircle size={20}/></div><BookOpen size={19}/><Bot size={20}/><div className="rail-bottom"><Avatar className="size-7"><AvatarFallback>S</AvatarFallback></Avatar></div></aside>
        <aside className="inbox-nav"><h2>Support <ChevronDown size={14}/></h2>{[{ label: 'Inbox', icon: Inbox },{ label: 'Mine', icon: User },{ label: 'Needs review', icon: Shield },{ label: 'Resolved', icon: Check }].map(({ label, icon: Icon }) => <button key={label} className={filter === label ? 'nav-selected' : ''} onClick={() => setFilter(label)}><Icon size={16}/>{label}<span>{label === 'Resolved' ? Number(resolved) : label === 'Mine' ? Number(assigned) : 1}</span></button>)}<div className="nav-section">TEAM INBOXES</div><div className="team-row"><span className="team-dot"/>Customer support</div></aside>
        <div className="conversation-list"><div className="list-title"><strong>{filter}</strong><SlidersHorizontal size={15}/></div><label className="list-search"><Search size={14}/><input aria-label="Search conversations" placeholder="Search" value={query} onChange={e => setQuery(e.target.value)}/></label>{shown ? <div className="selected-conversation"><div className="row-heading"><strong>Alex Morgan</strong><small>2m</small></div><div>Report of inappropriate content</div><span className="list-review"><Shield size={12}/> Needs review</span></div> : <div className="list-empty">No conversations</div>}<div className="list-hint">1 demo conversation</div></div>
        <div className="conversation"><header className="conversation-header"><Avatar className="size-8"><AvatarFallback>AM</AvatarFallback></Avatar><div><strong>Alex Morgan</strong><small>Website chat</small></div><div className="header-actions"><Button variant="outline" size="sm" onClick={() => setAssigned(value => !value)}>{assigned ? 'Assigned to you' : 'Assign to me'}</Button><Button size="icon-sm" variant={resolved ? 'default' : 'outline'} aria-label={resolved ? 'Reopen conversation' : 'Resolve conversation'} title={resolved ? 'Reopen conversation' : 'Resolve conversation'} onClick={() => setResolved(value => !value)}><Check/></Button></div></header>
          <div className="safety-strip"><Shield size={16}/><strong>AI paused</strong><span>Human review required</span><Tooltip><TooltipTrigger asChild><Button variant="ghost" size="icon-xs" aria-label="Why is AI paused?"><Info/></Button></TooltipTrigger><TooltipContent>The attachment is excluded from AI. Viewing it does not resume AI.</TooltipContent></Tooltip><button className="details-button" onClick={() => setDetails(value => !value)}>Details</button></div>
          {details && <div className="safety-details"><div><strong>Attachment flagged</strong><p>Potentially explicit imagery. The attachment is hidden and excluded from AI processing.</p><small>Viewing is a human-only action. This preview does not run a moderation service.</small></div><button aria-label="Close safety details" onClick={() => setDetails(false)}><X size={15}/></button></div>}
          <div className="thread" ref={threadRef} aria-label="Conversation messages"><div className="thread-date">Today</div>{messages.map(message => <Fragment key={message.id}>{message.id === 'held' ? <div className="attachment-row"><Attachment/></div> : <MessageBubble readOnly message={asSupportMessage(message)} customerDisplayName="Alex Morgan" teammateDisplayName={message.senderName || 'Support'} source="widget"/>}</Fragment>)}{assigned && <div className="thread-event"><User size={12}/> You assigned this conversation to yourself</div>}{resolved && <div className="thread-event"><Check size={12}/> Conversation resolved</div>}</div>
          <form className="reply-composer" onSubmit={e => { e.preventDefault(); send(draft, 'agent'); setDraft(''); }}><div className="reply-label">Reply <ChevronDown size={13}/><span>Visible to the customer</span></div><textarea aria-label="Reply to Alex" placeholder="Reply to Alex…" value={draft} onChange={e => setDraft(e.target.value)} onKeyDown={e => { if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); send(draft, 'agent'); setDraft(''); } }}/><div className="composer-footer"><span>⌘ / Ctrl + Enter to send</span><Button size="sm" type="submit" disabled={!draft.trim()}>Send<Send size={14}/></Button></div></form>
        </div>
        {view === 'inbox' && <aside className="contact-panel"><div><strong>Contact details</strong><PanelRightClose size={15}/></div><Avatar className="size-12"><AvatarFallback>AM</AvatarFallback></Avatar><strong>Alex Morgan</strong><small>alex@example.com</small><hr/><label>Assignee<span>{assigned ? 'You' : 'Unassigned'}</span></label><label>Status<span>{resolved ? 'Resolved' : 'Open'}</span></label><label>Channel<span>Website chat</span></label><label>AI status<span>Paused</span></label></aside>}
      </div></section>}
      {view !== 'inbox' && <section className="widget-panel" aria-label="Customer widget preview"><div className="panel-label"><MessageCircle size={14}/> Customer widget <span>Visitor view</span></div><Widget messages={messages} onSend={customerSend}/></section>}
    </main><footer className="preview-footer"><Shield size={13}/> Safe sample content · No messages are saved or sent to an AI service</footer>
  </div></TooltipProvider>;
}
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={queryClient}><App/></QueryClientProvider>);
