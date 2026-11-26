import { Inbox, PanelRight, Sparkles } from 'lucide-react';
import { SupportStory } from '../../_components/support-story/SupportStory';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'Find the next conversation', copy: 'Organize chat, email, Slack, and Telegram conversations into shared and team inboxes, with SLA timers and tags in view.', icon: Inbox },
  { title: 'See what happened before', copy: 'The AI handoff, the customer’s history, and the linked task are already in the conversation.', icon: PanelRight },
  { title: 'Reply with a clear next step', copy: 'Ask Agent explains the cause, so your reply tells the customer what happened and what to do.', icon: Sparkles },
];

export function SupportInboxShowcase() {
  return <figure className="support-inbox-showcase">
    <div className="support-inbox-stage"><SupportStory /></div>
    <figcaption><div className="support-inbox-callouts">{DETAILS.map(({ title, copy, icon: Icon }) => <div key={title} className="support-inbox-callout"><span className="support-inbox-callout-copy"><span className="support-inbox-callout-title"><Icon size={15} aria-hidden="true" />{title}</span><span>{copy}</span></span></div>)}</div></figcaption>
  </figure>;
}
