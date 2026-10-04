'use client';

import { useState } from 'react';
import { ArrowLeft, Bell, CalendarDays, Check, Circle, Flag, Heart, Layers, Pause, Play, RotateCcw, UserRound, Users, X } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowClick } from '../../_components/WorkflowParts';

const BEATS = [0, 1400, 3200, 5100, 6500, 8200, 10100, 11300];
const EPICS = [
  { title: 'Identity mapping', state: 'Done', progress: 100, color: '#8396b7' },
  { title: 'SSO pilot', state: 'In progress', progress: 67, color: '#ac98bf' },
  { title: 'Rollout readiness', state: 'In progress', progress: 33, color: '#b8a173' },
];

export function ProjectHealth() {
  const [paused, setPaused] = useState(false);
  const [replay, setReplay] = useState(0);
  const { container, phase, playing, reducedMotion, cycle } = useWorkflowPlayback({
    beats: BEATS, duration: 18500, paused, resetKey: replay,
  });
  const saved = phase >= 7;
  const logging = phase >= 4 && phase <= 6;
  const value = saved ? 100000 : 50000;
  const percent = value / 500000 * 100;
  return <div className="objective-demo" ref={container} data-phase={phase} data-playing={playing} role="region" aria-label="Helpin objective: increase ARR by $500k this quarter">
    <header className="pod-header">
      <div className="pod-breadcrumb"><ArrowLeft size={14}/><span>Objectives</span></div>
      <div className="pod-heading">
        <h3>Increase ARR by $500k this quarter.</h3>
        <div><span>Strategic objective</span><span className="pod-active"><i/>Active</span></div>
      </div>
      <div className="pod-actions">
        <span className="pod-follow"><Bell size={13}/>Follow</span>
        {!reducedMotion && <div className="pod-playback">
          <button type="button" aria-label={paused?'Play objective animation':'Pause objective animation'} aria-pressed={paused} onClick={()=>setPaused(previous=>!previous)}>{paused?<Play size={13}/>:<Pause size={13}/>}</button>
          <button type="button" aria-label="Replay objective animation" onClick={()=>{setPaused(false);setReplay(previous=>previous+1);}}><RotateCcw size={13}/></button>
        </div>}
        <span className="pod-saved"><Check size={11}/>Saved</span>
      </div>
    </header>
    <div className="pod-body" key={cycle}>
      <div className="pod-main">
        <p className="pod-description">Grow enterprise revenue by removing the blockers to a paid rollout.</p>
        <section className="pod-progress pod-reveal" data-visible={phase>=1} aria-hidden={phase<1} aria-label="Objective progress">
          <h4>Progress</h4>
          <div className="pod-metrics">
            <div><span>Epic progress</span><strong className="pod-positive">67%</strong><progress value={6} max={9} aria-label="6 of 9 linked tasks completed"/><small>6 of 9 linked tasks completed</small></div>
            <div data-updated={saved}><span>Key results</span><strong>{percent}%</strong><progress value={value} max={500000} aria-label="Revenue target achieved"/><small>1 measurable outcome</small></div>
            <div><span>Target date</span><strong className="pod-target-date">Dec 31, 2026</strong><progress className="pod-time-progress" value={50} max={100} aria-label="Half of the quarter elapsed"/><small>46 days remaining</small></div>
          </div>
          <p className="pod-progress-note">67% of work is done and <b>{percent}% of outcomes achieved.</b></p>
        </section>
        <section className="pod-key-results pod-reveal" data-visible={phase>=1} data-focused={phase===1||phase===3||saved} aria-hidden={phase<1} aria-label="Key results">
          <div className="pod-section-heading"><h4>Key results <span>1</span></h4><span className="pod-text-action">+ Add key result</span></div>
          <div className="pod-key-result">
            <div className="pod-result-name"><strong>New annual recurring revenue (USD)</strong><span>{percent}% achieved <i/>Behind</span><small>Sam Rivera changed the current value to {value.toLocaleString('en-US')} <i/>{saved?'Just now':'Nov 8'}</small></div>
            <div className="pod-result-values">
              <dl><div><dt>At start</dt><dd>0</dd></div><div data-updated={saved}><dt>Current</dt><dd>{value.toLocaleString('en-US')}</dd></div><div><dt>Target</dt><dd>500,000</dd></div></dl>
              <span className="pod-log-action">Log result{phase===3&&<WorkflowClick delay={100}/>}</span>
            </div>
          </div>
        </section>
        <section className="pod-epics pod-reveal" data-visible={phase>=2} data-focused={phase===2} aria-hidden={phase<2} aria-label="Linked epics">
          <div className="pod-section-heading"><h4>Linked epics <span>3</span></h4><span className="pod-text-action">+ Link epic</span></div>
          {EPICS.map(epic=><div className="pod-epic" key={epic.title}>
            <span className="pod-epic-icon" style={{color:epic.color}}><Layers size={15}/></span>
            <div><strong>{epic.title}</strong><small data-done={epic.progress===100}>{epic.state}</small></div>
            <span className="pod-epic-progress">{epic.progress}%<progress value={epic.progress} max={100} aria-label={epic.title+' progress'}/></span>
          </div>)}
        </section>
      </div>
      <aside className="pod-properties" aria-label="Objective properties">
        <div className="pod-property pod-state"><Circle size={13}/><span>State</span><strong className="pod-active">Active</strong></div>
        <div className="pod-property pod-health"><Heart size={13}/><span>Health</span><strong>At risk</strong></div>
        <hr/>
        <div className="pod-property pod-teams"><Users size={13}/><span>Teams</span><strong>Product, Sales</strong></div>
        <div className="pod-property pod-owner"><UserRound size={13}/><span>Owners</span><strong><img src="/new/avatars/sam.webp" width={19} height={19} alt=""/>Sam Rivera</strong></div>
        <hr/>
        <div className="pod-property pod-start"><CalendarDays size={13}/><span>Start date</span><strong>Oct 1, 2026</strong></div>
        <div className="pod-property pod-deadline"><Flag size={13}/><span>Target date</span><strong>Dec 31, 2026</strong></div>
      </aside>
      {logging&&<div className="pod-dialog-stage">
        <div className="pod-dialog" role="img" aria-label={phase>=5?'Logging a new current revenue value of 100,000':'Log result dialog showing current revenue of 50,000 and target of 500,000'}>
          <div className="pod-dialog-heading"><h4>Log result</h4><X size={16} aria-hidden="true"/></div>
          <p>New annual recurring revenue (USD)</p>
          <div className="pod-dialog-values"><span>Current <strong>50,000</strong></span><span>Target <strong>500,000</strong></span></div>
          <div className="pod-field" data-editing={phase>=5}><span>New current value</span><div key={phase>=5?'entered':'previous'}>{phase>=5?'100000':'50000'}{phase===5&&<i aria-hidden="true"/>}</div></div>
          <div className="pod-dialog-footer"><span>Cancel</span><span className="pod-log-confirm">Log result{phase===6&&<WorkflowClick delay={100}/>}</span></div>
        </div>
      </div>}
    </div>
  </div>;
}
