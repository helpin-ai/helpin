'use client';

import { useId, useState, type KeyboardEvent } from 'react';
import { ChevronDown, CircleCheck, ListChecks, Minus, Pause, Play, Plus } from 'lucide-react';
import { REVIEW_DRAFT, REVIEW_REQUEST, REVIEW_RUNS, REVIEW_STEPS, type ReviewRunId } from './ask-agent-demo';

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
        <div className="aaw-title"><h3>{composing ? 'New chat or task' : tab === 'chats' ? 'Rollout follow-up' : run.title}</h3><span>{tab === 'runs' && selected !== 'review' ? `${run.agent} · Specialist` : 'OrbitDesk · Ask Agent'}</span></div>
        <span className="aaw-status">{composing ? 'Example' : tab === 'chats' ? 'Draft' : done ? 'Done' : 'Working'}</span>
        <button className="aaw-view" type="button" aria-pressed={view === 'timeline'} onClick={() => { onInteract(); setView(value => value === 'summary' ? 'timeline' : 'summary'); }}>{view === 'timeline' ? 'Summary' : 'Timeline'}<ChevronDown size={12} /></button>
        <button className="aaw-icon-button" type="button" aria-label={`${paused ? 'Play' : 'Pause'} Ask Agent review animation`} aria-pressed={paused} onClick={onTogglePlayback}>{paused ? <Play size={15} /> : <Pause size={15} />}</button>
        <button className="aaw-icon-button" type="button" aria-label="Minimize Ask Agent preview" onClick={onMinimize}><Minus size={17} /></button>
      </header>
      <div className="aaw-content" id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-${tab}`} tabIndex={0}>
        {composing ? <div className="aaw-compose"><p className="aaw-label">Example request</p><h4>Start with the work you need done.</h4><p>{REVIEW_REQUEST}</p><button type="button" onClick={() => { setComposing(false); setTab('runs'); setSelected('review'); setView('summary'); onRestart(); }}><Play size={14} />Run example review</button><small>This demo uses prepared workspace context.</small></div>
          : tab === 'chats' ? <><p className="aaw-label">Sam Rivera</p><p>What update should I prepare for Maya?</p><div className="aaw-chat-answer"><img src="/brand/helpin-icon-ink.svg" width={23} height={23} alt="" /><div><h4>Ask Agent</h4><p>Prepare a status update from the findings. Don’t announce a release until the team has confirmed it.</p><div className="aaw-draft"><span>Maya Chen · Northstar Labs · Ready for Sam’s review · Not sent</span><p>{REVIEW_DRAFT}</p></div></div></div></>
          : view === 'timeline' ? <><p className="aaw-label">Execution timeline</p><h4>One request. Four focused checks.</h4><p>Customer history first. Engineering and docs checks run in parallel. The follow-up uses their findings.</p><WorkPlan phase={phase} /></>
          : selected === 'review' ? <>
            <p className="aaw-label">Request</p><p className="aaw-request">{REVIEW_REQUEST}</p>
            <p className="aaw-label">Sam Rivera · Maya’s conversation · Rollout review · EXP-142 · Renewal notes</p>
            <p className="aaw-summary">I’ll bring the findings together and use the existing export task, EXP-142.</p>
            <div className="aaw-findings"><h4>Specialist findings</h4><ul>{REVIEW_RUNS.slice(1).map(item => <li key={item.id} data-ready={phase >= item.completeAt}><strong>{item.agent}</strong> · {item.detail}</li>)}</ul></div>
            <div className="aaw-result"><h4>Here’s what needs to happen next.</h4><p>Continue the work in EXP-142 rather than creating another task.</p><p>Prepare a pagination fix and a regression test for exports beyond 10,000 contacts. Update the troubleshooting guide with the details customers should share when records are missing.</p><p>Maya also needs a status update. Don’t announce a release until the team has confirmed it.</p></div>
            {phase >= 5 && <div className="aaw-draft"><span>Maya Chen · Northstar Labs</span><p style={{ whiteSpace: 'pre-line' }}>{REVIEW_DRAFT}</p><small>Ready for Sam’s review · Not sent</small></div>}
            <WorkPlan phase={phase} />
          </> : <><p className="aaw-label">{run.agent} · {run.role}</p><h4>{run.title}</h4><p className="aaw-specialist-finding">{run.detail}</p><div className="aaw-run-result"><CircleCheck size={17} /><span>Findings returned to Ask Agent’s rollout review.</span></div><button className="aaw-back" type="button" onClick={() => selectRun('review')}>Back to rollout review</button></>}
      </div>
    </div>
  </div>;
}
