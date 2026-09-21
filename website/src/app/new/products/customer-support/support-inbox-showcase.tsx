'use client';

import { useState } from 'react';
import { Inbox, PanelRight, Sparkles } from 'lucide-react';
import { SupportWorkspace } from './support-workspace';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'Find the next conversation.', copy: 'Organize chat and email into shared and team inboxes.', icon: Inbox },
  { title: 'See what happened before.', copy: 'Review the customer’s history and the work already underway.', icon: PanelRight },
  { title: 'Reply with a clear next step.', copy: 'Use the findings to explain what happens next.', icon: Sparkles },
];

export function SupportInboxShowcase() {
  const [selected, setSelected] = useState<number | null>(null);
  const [preview, setPreview] = useState<number | null>(null);
  const active = preview ?? selected;
  return <figure className="support-inbox-showcase">
    <div className="support-inbox-stage"><SupportWorkspace highlight={active} /></div>
    <figcaption><p className="support-demo-caption">Customer history, investigation notes, and work in progress—beside the conversation.</p><div className="support-inbox-callouts">{DETAILS.map(({ title, copy, icon: Icon }, index) => <button key={title} type="button" aria-pressed={selected === index} aria-label={`Highlight area ${index + 1}: ${title}`} data-active={active === index} onMouseEnter={() => setPreview(index)} onMouseLeave={() => setPreview(null)} onFocus={() => setPreview(index)} onBlur={() => setPreview(null)} onClick={() => setSelected(value => value === index ? null : index)}><span className="support-inbox-callout-number">{String(index + 1).padStart(2, '0')}</span><span className="support-inbox-callout-copy"><span className="support-inbox-callout-title"><Icon size={15} aria-hidden="true" />{title}</span><span>{copy}</span></span></button>)}</div></figcaption>
  </figure>;
}
