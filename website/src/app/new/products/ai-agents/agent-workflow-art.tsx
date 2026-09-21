'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { BookOpen, Check, Code2, GitPullRequest, ListChecks, MessageSquare, Pause, Play, Plug, Search, ShieldCheck } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

type Variant = 'context' | 'coordination' | 'tools' | 'coding' | 'approval';
const LABELS: Record<Variant, string> = {
  context: 'workspace context', coordination: 'agent coordination', tools: 'connected tools', coding: 'coding workflow', approval: 'agent approval',
};
const DESCRIPTIONS: Record<Variant, string> = {
  context: 'Illustrative OrbitDesk workflow: a conversation, export guide, and linked task establish that Maya’s export stops at 10,000 of 18,400 rows. EXP-142 is in progress; the earlier smaller export did not meet the customer’s need.',
  coordination: 'Illustrative agent plan: review Maya’s export issue, run the coding agent’s investigation and the docs agent’s knowledge review in parallel, then combine both findings into a fix plan.',
  tools: 'Illustrative external MCP connection: issue lookup and issue search are selected for the agent. Issue editing is not selected. Ask Agent retrieves EXP-142’s in-progress status.',
  coding: 'Illustrative coding workflow: EXP-142 links Maya’s report to a proposed pagination fix and a regression test. The changes are ready for Sam to review and have not been merged.',
  approval: 'Illustrative approval checkpoint: an agent requests approval to add findings to existing task EXP-142. Sam approves only that update. The customer message still awaits review.',
};
function Step({ children, at = 0, className = '' }: { children: ReactNode; at?: number; className?: string }) {
  return <div className={`awa-step ${className}`} style={{ '--awa-delay': `${at}s` } as CSSProperties}>{children}</div>;
}
function AgentMark() { return <span className="awa-agent-mark"><img src="/brand/helpin-icon-white.svg" width={17} height={17} alt="" /></span>; }
function Context() {
  return <>
    <div className="awa-question">What’s holding up Maya’s export, and what have we already tried?</div>
    <div className="awa-source-list">
      <Step at={.35}><MessageSquare size={15} /><span>Maya’s conversation</span><Check size={13} /></Step>
      <Step at={.85}><BookOpen size={15} /><span>Export guide</span><Check size={13} /></Step>
      <Step at={1.35}><ListChecks size={15} /><span>Task EXP-142</span><Check size={13} /></Step>
    </div>
    <Step at={2.1} className="awa-answer"><div className="awa-answer-author"><AgentMark /><strong>Ask Agent</strong></div><p>Maya needs all 18,400 contacts. A smaller export worked, but it did not resolve that need. EXP-142 tracks the incomplete export and is marked In progress.</p><span className="awa-footnote">Earlier attempt and current work identified</span></Step>
  </>;
}
function Coordination() {
  return <>
    <div className="awa-plan-start"><ListChecks size={17} /><span>Investigate the export issue</span></div>
    <div className="awa-branch" aria-hidden="true" />
    <span className="awa-parallel-label">IN PARALLEL</span>
    <div className="awa-parallel">
      <Step at={.7}><img src="/new/agents/forge.svg" width={28} height={28} alt="" /><strong>Coding agent · Forge</strong><span>Check pagination</span><small><Check size={12} />Finding ready</small></Step>
      <Step at={.7}><img src="/new/agents/quill.svg" width={28} height={28} alt="" /><strong>Docs agent · Quill</strong><span>Review troubleshooting guidance</span><small><Check size={12} />Finding ready</small></Step>
    </div>
    <div className="awa-merge" aria-hidden="true" />
    <Step at={2.8} className="awa-plan-result"><AgentMark /><div><strong>Investigation complete · Plan ready for review</strong><p>Retrieve the remaining pages, test the full export, and add the missing troubleshooting guidance.</p></div></Step>
  </>;
}
function Tools() {
  return <>
    <div className="awa-tool-server"><Plug size={18} /><div><strong>Issue tracker</strong><span>External MCP · Connected</span></div><span className="awa-dot" /></div>
    <div className="awa-tool-list">
      <Step at={.4}><span className="awa-selected"><Check size={12} /></span><span>Look up an issue</span><small>Selected</small></Step>
      <Step at={.8}><span className="awa-selected"><Check size={12} /></span><span>Search issues</span><small>Selected</small></Step>
      <div><span className="awa-unselected" /><span>Edit an issue</span><small>Not selected</small></div>
    </div>
    <Step at={2} className="awa-tool-result"><Search size={17} /><div><span>EXP-142</span><strong>In progress</strong><p>Status checked alongside the customer history.</p></div></Step>
  </>;
}
function Coding() {
  return <>
    <div className="awa-code-task"><span>EXP-142</span><strong>Fix incomplete CSV exports</strong><small><MessageSquare size={12} />Maya’s conversation attached</small><p>Northstar needs all 18,400 contacts. Exporting a smaller selection is not a complete solution.</p><p>Retrieve the remaining pages before completing the export.</p></div>
    <Step at={.6} className="awa-code-file"><div><Code2 size={14} />export-contacts.ts<span>Proposed change</span></div><pre><code><span className="awa-code-remove">− return firstPage.rows;</span><span className="awa-code-add">+ const rows = [...firstPage.rows];</span><span className="awa-code-add">+ while (nextCursor) {'{'}</span><span className="awa-code-add">+   await appendNextPage(rows);</span><span className="awa-code-add">+ {'}'}</span><span className="awa-code-add">+ return rows;</span></code></pre></Step>
    <Step at={2} className="awa-code-test"><Check size={15} /><span>Regression test · Exports beyond 10,000 contacts</span></Step>
    <Step at={2.5} className="awa-answer"><strong>Lens · Code reviewer</strong><h4>Check the result against the request.</h4><p>The proposed change retrieves the remaining pages. Review the full-export test and confirm behavior when a page request fails.</p></Step><Step at={3} className="awa-code-review"><GitPullRequest size={18} /><div><strong>Changes ready for review</strong><span>Assigned to Sam · Not merged</span></div><img src="/new/avatars/sam.webp" width={28} height={28} alt="" /></Step>
  </>;
}
function Approval() {
  return <>
    <div className="awa-policy"><ShieldCheck size={16} /><span>Approval required for this action</span></div>
    <Step at={.5} className="awa-approval-request"><AgentMark /><h3>Add the investigation to EXP-142?</h3><p>Attach the support findings, code investigation, and documentation gap to the existing export task.</p><p className="awa-proposed-update">Northstar needs the full list of 18,400 contacts. The smaller-export workaround is insufficient. Investigate retrieving the remaining pages, add regression coverage, and update the troubleshooting guidance.</p><div className="awa-fake-actions"><span>Approve update</span><span>Cancel</span></div></Step>
    <Step at={2.8} className="awa-approved"><span className="awa-approval-tick"><Check size={17} /></span><img src="/new/avatars/sam.webp" width={26} height={26} alt="" /><div><strong>Approved by Sam</strong><span>Investigation added to EXP-142.</span></div></Step><p className="awa-footnote">Customer message still awaiting review.</p>
  </>;
}
export function AgentWorkflowArt({ variant }: { variant: Variant }) {
  const { container, playing, cycle } = useBentoPlayback(10000);
  const [paused, setPaused] = useState(false);
  return <div ref={container} className={`awa-art awa-${variant}`} data-playing={playing && !paused}>
    <div className="awa-toolbar"><span><span className="awa-workspace-mark">O</span>OrbitDesk<span className="awa-toolbar-slash">/</span>{variant === 'coding' ? 'Coding agent · Forge' : 'Ask Agent'}</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${LABELS[variant]} animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div className="awa-scene" key={cycle} role="img" aria-label={DESCRIPTIONS[variant]}><div aria-hidden="true">{variant === 'context' ? <Context /> : variant === 'coordination' ? <Coordination /> : variant === 'tools' ? <Tools /> : variant === 'coding' ? <Coding /> : <Approval />}</div></div>
  </div>;
}
