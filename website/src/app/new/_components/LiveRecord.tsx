'use client';

import { useCallback, useEffect, useState } from 'react';
import { Chip, Frame } from './ui';
import { useReviewNotes } from './ReviewNotes';

// Animated concept for the hero: the Acme Corp record fills in as the week happens.
// Tabs and activity filters mirror frontend/src/pages/crm/CompanyDetail.tsx and
// components/crm/ActivityTimeline.tsx. Everything is rendered from the first paint;
// steps only reveal rows and flip status text, so the frame is readable at rest.
const TABS = ['Overview', 'Tasks', 'Emails', 'Meetings', 'Calls', 'Deals', 'Support', 'Notes'];
const FILTERS = ['All', 'Notes', 'Emails', 'Calls', 'Meetings', 'Tasks', 'Deals', 'Support'];

const LAST_STEP = 8;
const STEP_MS = 2000;
const START_MS = 900;

type Row = { ic: string; title: string; when: string; at: number; sub: (step: number) => string; flipAt?: number };

const ROWS: Row[] = [
  { ic: 'MTG', title: 'Renewal call · Google Meet', when: 'Mon 14:00', at: 1, sub: () => 'Summary, transcript, 3 next steps. Action item accepted as a task.' },
  { ic: 'SUP', title: 'Maya R.: "Does SSO work with Okta?"', when: 'Tue 10:42', at: 2, sub: () => 'Website chat · answered by Sam · source: Set up SSO' },
  { ic: 'HLP', title: 'HLP-142 · Verify and document Okta SAML mapping', when: 'Tue 11:05', at: 4, flipAt: 5,
    sub: (s) => (s >= 5 ? 'In review · PR #318 open · created from the conversation' : 'In progress · Sam K. · created from the conversation') },
  { ic: 'DOC', title: 'Set up SSO with Okta', when: 'Thu 16:20', at: 6, flipAt: 7,
    sub: (s) => (s >= 7 ? 'Help center · drafted from a coverage gap · published' : 'Help center · draft from a coverage gap · in review') },
  { ic: 'DEAL', title: 'Growth renewal · $48k', when: 'Fri 09:30', at: 8, sub: () => 'Proposal → Renewal signed · Sam K.' },
];

export function LiveRecord() {
  const [step, setStep] = useState(1);
  const [run, setRun] = useState(0);
  const { shown } = useReviewNotes();

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setStep(LAST_STEP);
      return;
    }
    let current = 1;
    setStep(current);
    const start = window.setTimeout(() => {
      const tick = window.setInterval(() => {
        current += 1;
        setStep(current);
        if (current >= LAST_STEP) window.clearInterval(tick);
      }, STEP_MS);
      timers.push(tick);
    }, START_MS);
    const timers: number[] = [start];
    return () => timers.forEach((t) => { window.clearTimeout(t); window.clearInterval(t); });
  }, [run]);

  const replay = useCallback(() => setRun((r) => r + 1), []);

  return (
    <div className="hero-frame">
      <Frame crumb="CRM · Companies · Acme Corp" label="Acme Corp customer record with one timeline across meetings, support, tasks, docs, and deals">
        <div className="record gridlines">
          <div>
            <div className="rec-head">
              <div className="av">AC</div>
              <div><b>Acme Corp</b><span>Growth plan · renewal in 22 days · owner Sam K.</span></div>
            </div>
            <div className="tabs">{TABS.map((t, i) => <span key={t} className={i === 0 ? 'on' : undefined}>{t}</span>)}</div>
            <div className="filters">{FILTERS.map((f, i) => <span key={f} className={i === 0 ? 'on' : undefined}>{f}</span>)}</div>
            <div className="tl" aria-live="polite">
              {ROWS.map((r) => {
                const on = step >= r.at;
                const flipped = r.flipAt !== undefined && step >= r.flipAt;
                return (
                  <div key={r.title} className={on ? undefined : 'pending'} aria-hidden={!on}>
                    <div className="ic">{r.ic}</div>
                    <div><b>{r.title}</b><span key={flipped ? 'b' : 'a'} className={flipped ? 'flip' : undefined}>{r.sub(step)}</span></div>
                    <div className="when">{r.when}</div>
                  </div>
                );
              })}
            </div>
          </div>
          <div className="rail">
            <div className="ui-label">Signals</div>
            <div className="sig">
              <div className={step >= 1 ? undefined : 'pending'}><b>Timeline identified</b><span>“Okta migration before the renewal” <span className="src">Open meeting</span></span></div>
              <div className={step >= 2 ? undefined : 'pending'}><b>Relationship risk</b><span>Security sign-off pending <span className="src">View ticket</span></span></div>
            </div>
            <div className="ui-label">Next step</div>
            <div className="sig">
              <div className={step >= 3 ? undefined : 'pending'}>
                <b>{step >= 8 ? 'Follow up · done' : 'Follow up'}</b>
                <span>{step >= 8 ? 'Okta confirmed, renewal signed' : <>Confirm Okta support before renewal · <span className="src">Accept</span> · Dismiss</>}</span>
              </div>
            </div>
            <div className="ui-label">Linked tasks</div>
            <div className="sig">
              <div className={step >= 4 ? undefined : 'pending'}>
                <Chip tone="em">{step >= 5 ? 'HLP-142 · In review' : 'HLP-142 · In progress'}</Chip>
              </div>
            </div>
          </div>
        </div>
      </Frame>
      {shown ? (
        <div className="replay-row">
          <button type="button" className="replay" onClick={replay}>Replay animation</button>
          <span>step {step} of {LAST_STEP}</span>
        </div>
      ) : null}
    </div>
  );
}
