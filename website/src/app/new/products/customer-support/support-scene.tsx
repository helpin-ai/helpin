'use client';

import { useState, type CSSProperties } from 'react';
import { BookOpen, Check, CheckCheck, FileText, GitPullRequest, MessageSquare, Pause, Play, UserRound } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

type Variant = 'answer' | 'handoff' | 'followup';
const CONTENT = {
  answer: {
    title: 'An answer with a source',
    steps: [
      { icon: MessageSquare, label: 'Maya asks', text: 'Can we start with an admin-only pilot?' },
      { icon: BookOpen, label: 'Knowledge found', text: 'Okta setup guide', detail: 'Product knowledge' },
      { icon: FileText, label: 'Draft ready for review', text: 'Yes. Let’s start with your admins. Here’s the setup guide.', detail: 'Helpin AI draft' },
    ],
    status: 'Ready for your review',
  },
  handoff: {
    title: 'A handoff with the history',
    steps: [
      { icon: MessageSquare, label: 'Maya needs a teammate', text: 'Can someone check our role mapping?' },
      { icon: FileText, label: 'Internal note', text: 'Admin pilot planned. Role mapping needs a review.', detail: 'Context stays in the thread' },
      { icon: UserRound, label: 'Assigned to Sam Rivera', text: 'Conversation, notes, and customer details included.', detail: 'Your team takes it from here' },
    ],
    status: 'Same thread. No starting over.',
  },
  followup: {
    title: 'A request connected to the work',
    steps: [
      { icon: MessageSquare, label: 'Customer request', text: 'We need group-to-role mapping for rollout.' },
      { icon: GitPullRequest, label: 'Linked to product work', text: 'SSO Enterprise Readiness', detail: 'Original conversation attached' },
      { icon: CheckCheck, label: 'When the work is ready', text: 'Prepare the update for Maya and her team.', detail: 'Follow-up draft' },
    ],
    status: 'The customer stays connected',
  },
} as const;

export function SupportScene({ variant }: { variant: Variant }) {
  const { container, playing, cycle } = useBentoPlayback(8000);
  const [paused, setPaused] = useState(false);
  const content = CONTENT[variant];

  return <div className={`support-scene support-scene-${variant}`} ref={container} data-playing={playing && !paused}>
    <div className="support-scene-toolbar"><span>EXAMPLE WORKFLOW</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} animation: ${content.title}`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div className="support-scene-steps" key={cycle} role="img" aria-label={`${content.title}. ${content.steps.map(step => `${step.label}: ${step.text}`).join(' ')}. ${content.status}.`}>
      {content.steps.map(({ icon: Icon, label, text, ...rest }, index) => <div aria-hidden="true" className="support-scene-step" key={label} style={{ '--step-delay': `${index * 1.15}s` } as CSSProperties}><div className="support-scene-icon"><Icon size={16} strokeWidth={1.5} /></div><div className="support-scene-body"><span>{label}</span><p>{text}</p>{'detail' in rest && <small>{rest.detail}</small>}</div><Check className="support-scene-check" size={13} /></div>)}
      <div className="support-scene-result" aria-hidden="true"><CheckCheck size={14} /> {content.status}</div>
    </div>
  </div>;
}
