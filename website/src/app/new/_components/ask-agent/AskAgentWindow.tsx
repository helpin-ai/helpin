'use client';

import { useId, useState, type KeyboardEvent } from 'react';
import { ChevronDown, CircleCheck, ListChecks, Minus, Pause, Play, Plus } from 'lucide-react';
import { REVIEW_REQUEST, REVIEW_RUNS, REVIEW_STEPS, type ReviewRunId } from './ask-agent-demo';

type WindowProps = {
  phase: number;
  paused: boolean;
  onInteract: () => void;
  onTogglePlayback: () => void;
  onMinimize: () => void;
  onRestart: () => void;
};

function WorkPlan({ phase }: { phase: number }) {
  return <div className="aaw-plan">
    <div className="aaw-plan-heading"><ListChecks size={15} /><span>Work plan</span><small>{phase}/5 steps</small></div>
    <ol>{REVIEW_STEPS.map((step, index) => <li key={step} data-complete={phase > index} data-current={phase === index}>
      <CircleCheck size={17} aria-hidden="true" /><span>{step}</span><span className="aaw-sr-only">{phase > index ? ' — completed' : phase === index ? ' — in progress' : ' — queued'}</span>
    </li>)}</ol>
  </div>;
}

/** A local, presentational counterpart to DockRoster and DockRunView. */
export function AskAgentWindow({ phase, paused, onInteract, onTogglePlayback, onMinimize, onRestart }: WindowProps) {
  const id = useId();
  const [tab, setTab] = useState<'runs' | 'chats'>('runs');
  const [selected, setSelected] = useState<ReviewRunId>('review');
  const [view, setView] = useState<'summary' | 'timeline'>('summary');
  const [composing, setComposing] = useState(false);
  const run = REVIEW_RUNS.find(item => item.id === selected)!;
  const done = phase >= run.completeAt;

  function selectRun(value: ReviewRunId) { onInteract(); setSelected(value); setComposing(false); setTab('runs'); }
  function selectTab(value: 'runs' | 'chats') { onInteract(); setTab(value); setComposing(false); }
  function tabKey(event: KeyboardEvent<HTMLButtonElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const next = event.key === 'Home' ? 'runs' : event.key === 'End' ? 'chats' : tab === 'runs' ? 'chats' : 'runs';
    selectTab(next);
    document.getElementById(`${id}-${next}`)?.focus();
  }

  return <div className="aaw-window" role="region" aria-label="Ask Agent rollout review demo">
    <aside className="aaw-roster" aria-label="Example agent conversations">
      <div className="aaw-roster-controls">
        <button className="aaw-new" type="button" onClick={() => { onInteract(); setComposing(true); }}><Plus size={16} />New chat or task<small>Demo</small></button>
        <div className="aaw-tabs" role="tablist" aria-label="Ask Agent views">
          {(['runs', 'chats'] as const).map(value => <button key={value} type="button" role="tab" id={`${id}-${value}`} aria-controls={`${id}-panel`} aria-selected={tab === value} tabIndex={tab === value ? 0 : -1} onKeyDown={tabKey} onClick={() => selectTab(value)}>{value === 'runs' ? 'Agent runs' : 'Chats'}</button>)}
        </div>
      </div>
      <div className="aaw-roster-list">
        <p className="aaw-label">Today</p>
        {tab === 'runs' ? REVIEW_RUNS.map(item => <button type="button" key={item.id} className="aaw-run" aria-pressed={selected === item.id && !composing} onClick={() => selectRun(item.id)}>
          <img src={item.icon} width={25} height={25} alt="" /><span><strong>{item.title}</strong><small>{item.role} · {phase >= item.completeAt ? 'Done' : phase + 1 >= item.completeAt ? 'Working' : 'Queued'}</small></span>
        </button>) : <button type="button" className="aaw-run" aria-pressed={!composing} onClick={() => { onInteract(); setComposing(false); }}><img src="/new/avatars/sam.webp" width={25} height={25} alt="" /><span><strong>Rollout follow-up</strong><small>Sam Rivera · Example chat</small></span></button>}
      </div>
      {tab === 'runs' && <label className="aaw-mobile-run">Agent run<select value={selected} onChange={event => selectRun(event.target.value as ReviewRunId)}>{REVIEW_RUNS.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}</select></label>}
    </aside>
    <div className="aaw-main">
      <header className="aaw-header">
        <img src={tab === 'runs' ? run.icon : '/brand/helpin-icon-ink.svg'} width={27} height={27} alt="" />
        <div className="aaw-title"><h3>{composing ? 'New chat or task' : tab === 'chats' ? 'Rollout follow-up' : run.title}</h3><span>{tab === 'runs' && selected !== 'review' ? `${run.agent} · Specialist` : 'Workspace'}</span></div>
        <span className="aaw-status">{composing ? 'Example' : tab === 'chats' ? 'Draft' : done ? 'Done' : 'Working'}</span>
        <button className="aaw-view" type="button" aria-pressed={view === 'timeline'} onClick={() => { onInteract(); setView(value => value === 'summary' ? 'timeline' : 'summary'); }}>{view === 'timeline' ? 'Summary' : 'Timeline'}<ChevronDown size={12} /></button>
        <button className="aaw-icon-button" type="button" aria-label={`${paused ? 'Play' : 'Pause'} Ask Agent review animation`} aria-pressed={paused} onClick={onTogglePlayback}>{paused ? <Play size={15} /> : <Pause size={15} />}</button>
        <button className="aaw-icon-button" type="button" aria-label="Minimize Ask Agent preview" onClick={onMinimize}><Minus size={17} /></button>
      </header>
      <div className="aaw-content" id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-${tab}`} tabIndex={0}>
        {composing ? <div className="aaw-compose"><p className="aaw-label">Example request</p><h4>Start with the work you need done.</h4><p>{REVIEW_REQUEST}</p><button type="button" onClick={() => { setComposing(false); setTab('runs'); setSelected('review'); setView('summary'); onRestart(); }}><Play size={14} />Run example review</button><small>This demo uses prepared workspace context.</small></div>
          : tab === 'chats' ? <><p className="aaw-label">Sam Rivera</p><p>What should I send Maya before the security review?</p><div className="aaw-chat-answer"><img src="/brand/helpin-icon-ink.svg" width={23} height={23} alt="" /><div><h4>Ask Agent</h4><p>Share the Okta setup guide and the rollout checklist. Make the remaining group-mapping and SCIM work clear.</p><div className="aaw-draft"><span>Customer follow-up · Draft, not sent</span><p>{REVIEW_RUNS[4].detail}</p></div></div></div></>
          : view === 'timeline' ? <><p className="aaw-label">Execution timeline</p><h4>One request. Four focused checks.</h4><p>Customer history first. Engineering and docs checks run in parallel. The follow-up uses their findings.</p><WorkPlan phase={phase} /></>
          : selected === 'review' ? <>
            <p className="aaw-label">Request</p><p className="aaw-request">{REVIEW_REQUEST}</p>
            <p className="aaw-summary">{phase < 2 ? 'I’m reviewing the customer history and coordinating the engineering and docs checks in parallel.' : <>Northstar is waiting on Okta SSO.<br />I reviewed the customer history and coordinated the engineering and docs checks in parallel.</>}</p>
            <div className="aaw-findings"><h4>Findings</h4><ul><li data-ready={phase >= 1}>Maya needs Okta setup instructions before the security review.</li><li data-ready={phase >= 2}>Group mapping is in review. SCIM provisioning is still planned.</li><li data-ready={phase >= 3}>The setup guide needs group-mapping troubleshooting steps.</li></ul></div>
            <div className="aaw-result"><h4>Result</h4><p>{phase >= 5 ? 'The rollout checklist and Maya’s follow-up draft are ready for review.' : 'Preparing the rollout checklist and a customer follow-up for Sam to review.'}</p></div>
            <WorkPlan phase={phase} />
          </> : <><p className="aaw-label">{run.agent} · {run.role}</p><h4>{run.title}</h4><p className={selected === 'update' ? 'aaw-draft' : undefined}>{run.detail}</p><div className="aaw-run-result"><CircleCheck size={17} /><span>{selected === 'update' ? 'Draft ready for Sam’s review. No message has been sent.' : 'Findings returned to Ask Agent’s rollout review.'}</span></div><button className="aaw-back" type="button" onClick={() => selectRun('review')}>Back to rollout review</button></>}
      </div>
    </div>
  </div>;
}
