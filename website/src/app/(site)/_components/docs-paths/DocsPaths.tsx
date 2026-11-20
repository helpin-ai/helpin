'use client';

import { useState, type ReactNode } from 'react';
import {
  ArrowUpRight, Check, FileText, GitMerge, GitPullRequest, Loader2, MousePointer2, Pause, Play, Sparkles, Workflow, X,
} from 'lucide-react';
import { useWorkflowPlayback } from '../useWorkflowPlayback';
import './docs-paths.css';

// Two app-styled scenes for the homepage docs section. Both follow the real product:
// Support → Coverage Gaps (frontend/src/components/support/coverage) and the
// "Update docs from merged PRs" flow template (server/internal/templates/manifests/update_docs_from_merged_prs).

const QUILL = '/new/agents/quill.svg';

function Playback({ paused, onToggle }: { paused: boolean; onToggle: () => void }) {
  return <button type="button" className="dp-playback" onClick={onToggle} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}{paused ? 'Play' : 'Pause'}</button>;
}

function Frame({ title, children }: { title: ReactNode; children: ReactNode }) {
  return <div className="dp-frame"><div className="dp-bar"><span className="dp-logo">O</span><span className="dp-bar-title">{title}</span></div>{children}</div>;
}

// Stages: 0 gaps list · 1 cursor opens a gap · 2 gap details with grouped evidence
// 3 "Prepare a fix" clicked, Quill reads · 4 draft saved for review
const GAP_BEATS = [0, 1800, 2800, 6200, 9400];
const GAPS = [
  { title: 'Accented names look garbled in Excel', preview: 'Ü shows as Ã¼ after opening the CSV', kind: 'Missing knowledge', dot: 'blue', count: 9, seen: '2h ago' },
  { title: 'Exporting more than 10,000 contacts', preview: 'The export stops at 10,000 rows', kind: 'Incomplete knowledge', dot: 'blue', count: 6, seen: '5h ago' },
  { title: 'Where to find the API key', preview: 'The guide shows the old settings page', kind: 'Outdated knowledge', dot: 'blue', count: 4, seen: '1d ago' },
  { title: 'Change the billing email', preview: 'Needs an action the AI cannot take', kind: 'Action or workflow', dot: 'violet', count: 3, seen: '2d ago' },
];
const EVIDENCE = [
  { who: 'Maya Chen', text: 'Names with accents show up as garbled characters.' },
  { who: 'Jonas Weber', text: 'Müller becomes MÃ¼ller when I open the export.' },
];

export function CoverageGapsScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: GAP_BEATS, duration: 16500, paused });
  const open = phase >= 2;
  return <div className="dp-root" ref={container} data-playing={playing}>
    <Frame title="Support coverage">
      <div className="dp-coverage" key={cycle}>
        <div className="dp-tabs" aria-hidden="true"><span data-active="true">All gaps</span><span><i data-dot="blue" />Content gaps</span><span><i data-dot="amber" />Data gaps</span><span><i data-dot="violet" />Action gaps</span></div>
        <div className="dp-table">
          <div className="dp-thead"><span>Gap</span><span>Needed improvement</span><span>Impact · 30d</span><span>Last seen</span></div>
          {GAPS.map((gap, index) => <div key={gap.title} className="dp-row" data-selected={index === 0 && phase >= 1}>
            <span className="dp-row-main"><strong>{gap.title}</strong><small>{gap.preview}</small></span>
            <span className="dp-kind"><i data-dot={gap.dot} />{gap.kind}</span>
            <span className="dp-count">{gap.count} conversations</span>
            <span className="dp-seen">{gap.seen}</span>
          </div>)}
        </div>
        {phase === 1 && <MousePointer2 className="dp-cursor dp-cursor-row" size={18} aria-hidden="true" />}
        {open && <aside className="dp-detail" aria-label="Gap details">
          <header><strong>{GAPS[0].title}</strong><span className="dp-status">{phase >= 4 ? 'Draft ready' : 'Open'}</span><X size={14} aria-hidden="true" /></header>
          <div className="dp-detail-meta"><i data-dot="blue" />Missing knowledge · 9 conversations grouped</div>
          {phase < 3
            ? <ul className="dp-evidence">{EVIDENCE.map(item => <li key={item.who}><span className="dp-ev-avatar">{item.who.split(' ').map(part => part[0]).join('')}</span><span><b>{item.who}</b>“{item.text}”</span></li>)}<li className="dp-ev-more">+ 7 more conversations</li></ul>
            : <p className="dp-evidence-folded">{EVIDENCE.map(item => <span key={item.who} className="dp-ev-avatar">{item.who.split(' ').map(part => part[0]).join('')}</span>)}<span>Maya Chen, Jonas Weber, and 7 more</span></p>}
          <div className="dp-dock">
            {phase < 3 && <div className="dp-starters"><span className="dp-starter dp-starter-primary"><Sparkles size={12} />Prepare a fix</span><span className="dp-starter">Check the evidence</span><span className="dp-starter">Find existing guidance</span>{phase === 2 && <MousePointer2 className="dp-cursor dp-cursor-chip" size={18} aria-hidden="true" />}</div>}
            {phase >= 3 && <>
              <div className="dp-ask">Prepare a fix</div>
              <div className="dp-agent"><img src={QUILL} width={24} height={24} alt="" /><span><b>Quill</b> Documentation agent</span></div>
              {phase === 3 && <div className="dp-working"><Loader2 size={13} className="dp-spin" />Reading 9 conversations and the CSV export guide…</div>}
              {phase >= 4 && <div className="dp-draft">
                <div className="dp-draft-head"><FileText size={15} /><span><b>Open a CSV export in Excel</b><small>New article · Help Center · Draft</small></span></div>
                <ol><li>In Excel, choose Data → From Text/CSV.</li><li>Set File origin to 65001: Unicode (UTF-8).</li><li>Select Load. Accented names display correctly.</li></ol>
                <p className="dp-draft-note"><Check size={13} />Draft saved for review. Nothing is published until your team does it.</p>
              </div>}
            </>}
          </div>
        </aside>}
      </div>
    </Frame>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{['Coverage gaps lists grouped customer questions with how many conversations hit each one.', 'A teammate opens the top gap.', 'The gap shows the nine conversations behind it.', 'One click on Prepare a fix starts Quill, the documentation agent.', 'Quill saves a new help article as a draft for review.'][phase]}</span>
  </div>;
}

// Stages: 0 merged PR · 1 flow starts Quill · 2–5 run steps · 6 results · 7 waiting for review
const PR_BEATS = [0, 1400, 2600, 3600, 4600, 5600, 7000, 9600];
const STEPS = [
  'Read PR #482: 11 files changed, linked task EXP-142',
  'Skipped 4 files readers never see: tests and CI',
  'Searched the Help Center for “export” and “10,000 rows”',
  'Read the 3 matching articles in full',
];

export function MergedPrDocsScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: PR_BEATS, duration: 16500, paused });
  return <div className="dp-root" ref={container} data-playing={playing}>
    <Frame title="Update docs from merged PRs">
      <div className="dp-pr" key={cycle}>
        <div className="dp-pr-card">
          <span className="dp-merged"><GitMerge size={13} />Merged</span>
          <div><strong>Export every contact, not just the first 10,000</strong><small><GitPullRequest size={12} />#482 · orbitdesk/app · merged into main</small></div>
        </div>
        {phase >= 1 && <div className="dp-flow"><Workflow size={13} />Flow started <b>Quill</b> · Docs space: Help Center · Propose changes for review</div>}
        {phase >= 2 && <div className="dp-run">
          <div className="dp-agent"><img src={QUILL} width={24} height={24} alt="" /><span><b>Quill</b> Documentation agent</span><span className="dp-run-state">{phase >= 7 ? 'Done' : 'Working'}</span></div>
          <ul className="dp-steps">{STEPS.map((step, index) => phase >= index + 2 && <li key={step} data-done={phase > index + 2 || phase >= 6}>{phase > index + 2 || phase >= 6 ? <Check size={13} /> : <Loader2 size={13} className="dp-spin" />}{step}</li>)}</ul>
        </div>}
        {phase >= 6 && <div className="dp-results">
          <div className="dp-result" data-kind="proposed">
            <div className="dp-result-head"><FileText size={14} /><b>Export your contacts</b><span className="dp-pill dp-pill-amber">Change proposed</span></div>
            <p className="dp-diff"><del>Each export includes up to 10,000 contacts.</del><ins>Each export includes every contact that matches your filters.</ins></p>
          </div>
          <div className="dp-result"><div className="dp-result-head"><FileText size={14} /><b>Open a CSV export in Excel</b><span className="dp-pill">Checked · no change</span></div></div>
          <div className="dp-result"><div className="dp-result-head"><FileText size={14} /><b>Import contacts from a CSV</b><span className="dp-pill">Checked · no change</span></div></div>
        </div>}
        {phase >= 7 && <div className="dp-review"><span>1 change waiting for review · nothing published</span><span className="dp-review-btn">Review change<ArrowUpRight size={12} aria-hidden="true" /></span></div>}
      </div>
    </Frame>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{['Pull request 482 is merged into main.', 'The Update docs from merged PRs flow starts Quill.', 'Quill reads the changes and the linked task.', 'Quill skips tests and CI.', 'Quill searches the Help Center.', 'Quill reads the matching articles.', 'Quill proposes a change to one article and leaves two alone.', 'The change waits for review; nothing is published.'][phase]}</span>
  </div>;
}
