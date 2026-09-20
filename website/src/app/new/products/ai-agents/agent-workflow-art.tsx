'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { BookOpen, Check, Code2, GitPullRequest, ListChecks, MessageSquare, Pause, Play, Plug, Search, ShieldCheck } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

type Variant = 'context' | 'coordination' | 'tools' | 'coding' | 'approval';
const LABELS: Record<Variant, string> = {
  context: 'workspace context', coordination: 'agent coordination', tools: 'connected tools', coding: 'coding workflow', approval: 'agent approval',
};
const DESCRIPTIONS: Record<Variant, string> = {
  context: 'Illustrative OrbitDesk workflow: a conversation, export guide, and linked task establish that Maya’s export stops at 10,000 of 18,400 rows. EXP-142 is awaiting review.',
  coordination: 'Illustrative agent plan: review Maya’s export issue, run repository investigation and knowledge review in parallel, then combine both findings into a fix plan.',
  tools: 'Illustrative external MCP connection: issue lookup and issue search are selected for the agent. Issue editing is not selected. Ask Agent retrieves EXP-142’s review status.',
  coding: 'Illustrative coding workflow: EXP-142 links Maya’s report to a proposed pagination fix and a regression test. The changes are ready for Sam to review and have not been merged.',
  approval: 'Illustrative approval checkpoint: an agent requests approval to create a task for the incomplete CSV export. Sam approves the request, allowing the agent to continue.',
};
function Step({ children, at = 0, className = '' }: { children: ReactNode; at?: number; className?: string }) {
  return <div className={`awa-step ${className}`} style={{ '--awa-delay': `${at}s` } as CSSProperties}>{children}</div>;
}
function AgentMark() { return <span className="awa-agent-mark"><img src="/brand/helpin-icon-white.svg" width={17} height={17} alt="" /></span>; }
function Context() {
  return <>
    <div className="awa-question">What’s holding up Maya’s export?</div>
    <div className="awa-source-list">
      <Step at={.35}><MessageSquare size={15} /><span>Maya’s conversation</span><Check size={13} /></Step>
      <Step at={.85}><BookOpen size={15} /><span>Export guide</span><Check size={13} /></Step>
      <Step at={1.35}><ListChecks size={15} /><span>Task EXP-142</span><Check size={13} /></Step>
    </div>
    <Step at={2.1} className="awa-answer"><div className="awa-answer-author"><AgentMark /><strong>Ask Agent</strong></div><p>The export stops at 10,000 of 18,400 rows. A proposed fix is linked to EXP-142 and waiting for review.</p><span className="awa-footnote">Conversation + docs + task</span></Step>
  </>;
}
function Coordination() {
  return <>
    <div className="awa-plan-start"><ListChecks size={17} /><span>Investigate the export issue</span></div>
    <div className="awa-branch" aria-hidden="true" />
    <span className="awa-parallel-label">IN PARALLEL</span>
    <div className="awa-parallel">
      <Step at={.7}><Code2 size={20} /><strong>Engineering</strong><span>Inspect pagination</span><small><Check size={12} />Finding ready</small></Step>
      <Step at={.7}><BookOpen size={20} /><strong>Knowledge</strong><span>Review export docs</span><small><Check size={12} />Finding ready</small></Step>
    </div>
    <div className="awa-merge" aria-hidden="true" />
    <Step at={2.8} className="awa-plan-result"><AgentMark /><div><strong>One plan, both findings</strong><p>Fix pagination. Add a regression test. Review the troubleshooting guide.</p></div></Step>
  </>;
}
function Tools() {
  return <>
    <div className="awa-tool-server"><Plug size={18} /><div><strong>Issue tracker</strong><span>External MCP · Connected</span></div><span className="awa-dot" /></div>
    <div className="awa-tool-list">
      <Step at={.4}><span className="awa-selected"><Check size={12} /></span><span>Look up an issue</span><small>Read</small></Step>
      <Step at={.8}><span className="awa-selected"><Check size={12} /></span><span>Search issues</span><small>Read</small></Step>
      <div><span className="awa-unselected" /><span>Edit an issue</span><small>Not selected</small></div>
    </div>
    <Step at={2} className="awa-tool-result"><Search size={17} /><div><span>EXP-142</span><strong>Ready for review</strong><p>Available to Ask Agent alongside Maya’s customer history.</p></div></Step>
  </>;
}
function Coding() {
  return <>
    <div className="awa-code-task"><span>EXP-142</span><strong>Fix incomplete CSV exports</strong><small><MessageSquare size={12} />Maya’s conversation attached</small></div>
    <Step at={.6} className="awa-code-file"><div><Code2 size={14} />export-contacts.ts<span>Proposed change</span></div><pre><code><span className="awa-code-remove">− return firstPage.rows;</span><span className="awa-code-add">+ const rows = [...firstPage.rows];</span><span className="awa-code-add">+ while (nextCursor) {'{'}</span><span className="awa-code-add">+   await appendNextPage(rows);</span><span className="awa-code-add">+ {'}'}</span><span className="awa-code-add">+ return rows;</span></code></pre></Step>
    <Step at={2} className="awa-code-test"><Check size={15} /><span>Regression test: exports beyond 10,000 rows</span></Step>
    <Step at={3} className="awa-code-review"><GitPullRequest size={18} /><div><strong>Changes ready for review</strong><span>Assigned to Sam · Not merged</span></div><img src="/new/avatars/sam.webp" width={28} height={28} alt="" /></Step>
  </>;
}
function Approval() {
  return <>
    <div className="awa-policy"><ShieldCheck size={16} /><span>Approval required for this action</span></div>
    <Step at={.5} className="awa-approval-request"><AgentMark /><h3>Create a task for the export issue?</h3><p>Attach Maya’s conversation and the incomplete export details.</p><div className="awa-fake-actions"><span>Approve</span><span>Cancel</span></div></Step>
    <Step at={2.8} className="awa-approved"><span className="awa-approval-tick"><Check size={17} /></span><img src="/new/avatars/sam.webp" width={26} height={26} alt="" /><div><strong>Approved by Sam</strong><span>The agent can continue.</span></div></Step>
  </>;
}
export function AgentWorkflowArt({ variant }: { variant: Variant }) {
  const { container, playing, cycle } = useBentoPlayback(10000);
  const [paused, setPaused] = useState(false);
  return <div ref={container} className={`awa-art awa-${variant}`} data-playing={playing && !paused}>
    <div className="awa-toolbar"><span><span className="awa-workspace-mark">O</span>OrbitDesk<span className="awa-toolbar-slash">/</span>{variant === 'coding' ? 'Code Builder' : 'Ask Agent'}</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${LABELS[variant]} animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div className="awa-scene" key={cycle} role="img" aria-label={DESCRIPTIONS[variant]}><div aria-hidden="true">{variant === 'context' ? <Context /> : variant === 'coordination' ? <Coordination /> : variant === 'tools' ? <Tools /> : variant === 'coding' ? <Coding /> : <Approval />}</div></div>
  </div>;
}
