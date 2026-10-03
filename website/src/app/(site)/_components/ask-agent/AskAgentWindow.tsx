'use client';

import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUp, Check, CircleCheck, FileText, LoaderCircle, Mic, Minus, Paperclip, Pause, Play, Plus } from 'lucide-react';
import { CHAT_RUN, REVIEW_RUNS, RUN_COMPLETE_PHASE, type DemoTool, type DemoRunId } from './ask-agent-demo';

type WindowProps = {
  phase: number;
  streaming: boolean;
  playing: boolean;
  selected: DemoRunId;
  onSelectRun: (value: DemoRunId) => void;
  paused: boolean;
  onInteract: () => void;
  onTogglePlayback: () => void;
  onMinimize: () => void;
};

function StreamText({ text, animate, playing, onProgress }: { text: string; animate: boolean; playing: boolean; onProgress: () => void }) {
  const [length, setLength] = useState(0);
  const progress = useRef(0);
  useEffect(() => {
    if (!animate || !playing) return;
    let last = performance.now();
    const timer = window.setInterval(() => {
      const now = performance.now();
      progress.current = Math.min(text.length, progress.current + (now - last) / 14);
      last = now;
      const next = Math.floor(progress.current);
      setLength(next);
      if (next === text.length) window.clearInterval(timer);
    }, 40);
    return () => window.clearInterval(timer);
  }, [text, animate, playing]);
  useEffect(onProgress, [length, animate, onProgress]);
  return <span className="aaw-stream" data-streaming={animate && playing && length < text.length}>{animate ? text.slice(0, length) : text}</span>;
}

function Activity({ children }: { children: string }) {
  return <div className="aaw-activity" role="status"><span className="aaw-thinking-dots" aria-hidden="true"><i /><i /><i /></span>{children}</div>;
}

function ToolCall({ tool, complete }: { tool: DemoTool; complete: boolean }) {
  return <div className="aaw-tool" data-complete={complete} aria-label={`${tool.name}: ${complete ? 'complete' : 'working'}`}>
    <div className="aaw-tool-line">{complete ? <Check size={13} /> : <LoaderCircle className="aaw-tool-spinner" size={13} />}<span>{tool.label}</span><small>{complete ? 'Done' : 'Working'}</small></div>
    <div className="aaw-tool-source">{tool.source}<code title={tool.name}>{tool.name}</code></div>
    {complete && <p className="aaw-tool-result">{tool.result}</p>}
  </div>;
}

/** Presentational counterpart to the app's DockRoster, transcript, and DockInput. */
export function AskAgentWindow({ phase, streaming, playing, selected, onSelectRun, paused, onInteract, onTogglePlayback, onMinimize }: WindowProps) {
  const assistantName = 'Helpin AI';
  const id = useId();
  const tab = selected === 'chat' ? 'chats' : 'runs';
  const [composing, setComposing] = useState(false);
  const content = useRef<HTMLDivElement>(null);
  const run = selected === 'chat' ? CHAT_RUN : REVIEW_RUNS.find(item => item.id === selected)!;
  const done = phase >= RUN_COMPLETE_PHASE;
  const displayName = selected === 'review' || selected === 'chat' ? assistantName : run.agent;
  const followReply = useCallback(() => {
    // Follow output inside the illustration, without capturing page scrolling.
    if (content.current) content.current.scrollTop = content.current.scrollHeight;
  }, []);
  useEffect(() => {
    if (content.current) content.current.scrollTop = 0;
  }, [selected, tab, composing]);
  useEffect(() => {
    if (phase === 0) { if (content.current) content.current.scrollTop = 0; }
    else if (!composing) followReply();
  }, [phase, selected, tab, composing, followReply]);

  function selectRun(value: DemoRunId) { setComposing(false); onSelectRun(value); }
  function selectTab(value: 'runs' | 'chats') { selectRun(value === 'runs' ? 'review' : 'chat'); }
  function tabKey(event: KeyboardEvent<HTMLButtonElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const next = event.key === 'Home' ? 'runs' : event.key === 'End' ? 'chats' : tab === 'runs' ? 'chats' : 'runs';
    selectTab(next);
    document.getElementById(`${id}-${next}`)?.focus();
  }

  return <div className="aaw-window" role="region" aria-label={`${assistantName} agent runs demo`}>
    <aside className="aaw-roster" aria-label="Example agent conversations">
      <div className="aaw-roster-controls">
        <button className="aaw-new" type="button" onClick={() => { onInteract(); setComposing(true); }}><Plus size={16} />New chat or task<small>Demo</small></button>
        <div className="aaw-tabs" role="tablist" aria-label={`${assistantName} views`}>
          {(['runs', 'chats'] as const).map(value => <button key={value} type="button" role="tab" id={`${id}-${value}`} aria-controls={`${id}-panel`} aria-selected={tab === value} tabIndex={tab === value ? 0 : -1} onKeyDown={tabKey} onClick={() => selectTab(value)}>{value === 'runs' ? 'Agent runs' : 'Chats'}</button>)}
        </div>
      </div>
      <div className="aaw-roster-list">
        <p className="aaw-label">Today</p>
        {tab === 'runs' ? REVIEW_RUNS.map(item => <button type="button" key={item.id} className="aaw-run" aria-pressed={selected === item.id && !composing} onClick={() => selectRun(item.id)}>
          <img src={item.icon} width={25} height={25} alt="" /><span><strong>{item.title}</strong><small>{item.id === 'review' ? assistantName : `${item.agent} · ${item.role}`}{selected === item.id && ` · ${done ? 'Done' : paused ? 'Paused' : 'Working'}`}</small></span>
        </button>) : <button type="button" className="aaw-run" aria-pressed={!composing} onClick={() => selectRun('chat')}><img src="/new/avatars/sam.webp" width={25} height={25} alt="" /><span><strong>{CHAT_RUN.title}</strong><small>Sam Rivera · Example chat</small></span></button>}
      </div>
      {tab === 'runs' && <label className="aaw-mobile-run">Agent run<select value={selected} onChange={event => selectRun(event.target.value as DemoRunId)}>{REVIEW_RUNS.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}</select></label>}
    </aside>
    <div className="aaw-main">
      <header className="aaw-header">
        <img src={tab === 'runs' ? run.icon : '/brand/helpin-icon-ink.svg'} width={27} height={27} alt="" />
        <div className="aaw-title"><h3>{composing ? 'New chat or task' : run.title}</h3><span>{tab === 'runs' && selected !== 'review' ? `${run.agent} · ${run.role}` : `OrbitDesk · ${assistantName}`}</span></div>
        <span className="aaw-status">{composing ? 'Example' : done ? 'Done' : paused ? 'Paused' : 'Working'}</span>
        <button className="aaw-icon-button" type="button" aria-label={`Minimize ${assistantName} preview`} onClick={onMinimize}><Minus size={17} /></button>
      </header>
      <div ref={content} className="aaw-content" id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-${tab}`} tabIndex={0}>
        {composing ? <div className="aaw-compose"><p className="aaw-label">Example request</p><h4>Start with the work you need done.</h4><p>{CHAT_RUN.request}</p><button type="button" onClick={() => selectRun('chat')}><ArrowUp size={14} />Send example request</button><small>This demo uses prepared workspace context.</small></div>
          : <div key={selected} className="aaw-run-conversation">
            <div className="aaw-user-message"><p className="aaw-label">Sam Rivera</p><p className="aaw-request">{run.request}</p></div>
            <div className="aaw-response-label"><img src={run.icon} width={20} height={20} alt="" />{displayName}</div>
            {phase === 0 && <Activity>Thinking…</Activity>}
            {phase >= 1 && <p className="aaw-summary"><StreamText text={run.acknowledgement} animate={streaming} playing={playing} onProgress={followReply} /></p>}
            {phase >= 2 && <div className="aaw-tools" aria-label="Tool calls and context">{run.tools.map((tool, index) => phase >= index + 2 && <ToolCall key={tool.name} tool={tool} complete={phase > index + 2} />)}</div>}
            {phase >= 5 && <div className="aaw-result"><h4>{run.resultTitle}</h4><p className="aaw-stream-paragraph"><StreamText text={run.result} animate={streaming} playing={playing} onProgress={followReply} /></p></div>}
            {done && <div className="aaw-run-result"><CircleCheck size={15} /><span>{run.completion}</span></div>}
          </div>}
      </div>
      <div className="aaw-composer" aria-label="Helpin chat input preview">
        <div className="aaw-context-row"><span className="aaw-context-chip"><FileText size={11} />{run.context}</span><span className="aaw-add-context"><Plus size={11} />Add context</span></div>
        <div className="aaw-input-box">
          <textarea rows={1} readOnly aria-label="Message agent (demo)" placeholder={!done && !paused && !composing ? 'Agent is working…' : 'Message agent…'} onFocus={onInteract} />
          <div className="aaw-input-actions">
            <span className="aaw-input-decoration" title="Attach files in Helpin"><Paperclip size={15} /></span>
            <span className="aaw-composer-demo">Demo</span>
            <span className="aaw-input-decoration aaw-input-mic" title="Voice input in Helpin"><Mic size={15} /></span>
            {!composing && streaming ? <button className="aaw-playback" type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${assistantName} animation`} aria-pressed={paused} onClick={onTogglePlayback}>{paused ? <Play size={14} /> : <Pause size={14} />}</button> : <button className="aaw-send" type="button" disabled aria-label="Send message (demo)"><ArrowUp size={15} /></button>}
          </div>
        </div>
      </div>
    </div>
  </div>;
}
