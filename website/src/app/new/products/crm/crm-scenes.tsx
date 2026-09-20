'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { ArrowDown, ArrowRight, Building2, Check, CheckCheck, Circle, CircleDot, Clock3, FileText, GitBranch, Mail, MessageSquare, Pause, Play, ShieldCheck, Sparkles } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

function MotionFrame({ name, children }: { name: string; children: ReactNode }) {
  const { container, playing, cycle } = useBentoPlayback(9000);
  const [paused, setPaused] = useState(false);
  return <div ref={container} className="crm-motion" role="group" aria-label={name + " illustration"} data-playing={playing && !paused}>
    <div className="crm-demo-toolbar"><span><b className="crm-workspace-mark">O</b>OrbitDesk <i>/</i> {name}</span><button type="button" onClick={() => setPaused(!paused)} aria-label={`${paused ? 'Play' : 'Pause'} ${name.toLowerCase()} illustration`} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div key={paused ? 'paused' : cycle}>{children}</div>
  </div>;
}
function Step({ index, children, className = '' }: { index: number; children: ReactNode; className?: string }) {
  return <div className={`crm-motion-step ${className}`} style={{ '--crm-delay': `${index * 900 + 300}ms` } as CSSProperties}>{children}</div>;
}

export function CRMAccountScene() {
  return <div className="crm-hero-art"><div className="crm-art-orbit" aria-hidden="true" /><MotionFrame name="Customer context"><div className="crm-account-demo">
    <div className="crm-account-heading"><span className="crm-company-mark"><Building2 size={25} /></span><div><small>CUSTOMER RECORD</small><h2>Northstar Labs</h2><p>Maya Chen <span>·</span> Sam Rivera, owner</p></div><span className="crm-status">Customer</span></div>
    <div className="crm-account-facts"><div><small>Opportunity</small><strong>Enterprise rollout</strong></div><div><small>Next milestone</small><strong>Security approval</strong></div></div>
    <Step index={0} className="crm-source-note"><span className="crm-mini-label"><MessageSquare size={13} /> THE CUSTOMER SAYS</span><blockquote>“We can roll this out to the rest of the team once security approves SSO.”</blockquote><p><span className="crm-avatar">MC</span> Maya Chen <span>· Support conversation</span></p></Step>
    <div className="crm-account-connector" aria-hidden="true"><ArrowDown size={17} /></div>
    <Step index={1} className="crm-linked-task"><div><GitBranch size={16} /><span>SSO Enterprise Readiness</span><span className="crm-status crm-status-amber">In progress</span></div><strong>Validate group-to-role mapping</strong><p><CircleDot size={12} /> Linked task <span>· Sam Rivera</span></p></Step>
    <Step index={2} className="crm-next-step"><Sparkles size={17} /><div><small>PROPOSED NEXT STEP</small><p>Share the reviewed setup guide.</p></div><span className="crm-review-pill">For review</span></Step>
    <p className="crm-demo-caption">Illustrative customer workflow</p>
  </div></MotionFrame></div>;
}

const PIPELINES = [
  { label: 'New business', stages: ['Discovery', 'Evaluation', 'Decision'], deals: [
    { company: 'Acme Studio', name: 'Support workspace', amount: '$12,000', initials: 'AS', date: 'Oct 12', note: 'Confirm the support team’s requirements.', evidence: 'The discovery call identified shared inbox routing as the first priority.', source: 'Discovery meeting', stage: 0 },
    { company: 'Northstar Labs', name: 'Enterprise rollout', amount: '$24,000', initials: 'NL', date: 'Oct 16', note: 'Share the reviewed Okta setup guide.', evidence: 'Maya wants an admin-only pilot before the wider rollout. Group-to-role mapping is still being validated.', source: 'SSO rollout review · Meeting', stage: 1 },
    { company: 'Forma', name: 'Customer workspace', amount: '$18,000', initials: 'F', date: 'Oct 23', note: 'Confirm the procurement next step.', evidence: 'The team has reviewed the proposal and asked for a final procurement discussion.', source: 'Proposal follow-up · Email', stage: 2 },
  ] },
  { label: 'Renewals', stages: ['Review', 'Follow-up', 'Decision'], deals: [
    { company: 'Northstar Labs', name: 'Annual renewal', amount: '$24,000', initials: 'NL', date: 'Nov 01', note: 'Review the outstanding rollout blocker.', evidence: 'Maya wants to confirm the security checklist before discussing next year’s agreement.', source: 'Renewal discussion · Email', stage: 0 },
    { company: 'Forma', name: 'Workspace renewal', amount: '$18,000', initials: 'F', date: 'Nov 15', note: 'Send the revised renewal proposal for review.', evidence: 'The account owner recorded the team’s updated seat requirements after the review call.', source: 'Account review · Meeting', stage: 1 },
    { company: 'Acme Studio', name: 'Support renewal', amount: '$12,000', initials: 'AS', date: 'Nov 21', note: 'Confirm the customer’s decision date.', evidence: 'The customer has received the renewal terms and is completing its internal review.', source: 'Renewal terms · Email', stage: 2 },
  ] },
  { label: 'Expansion', stages: ['Interest', 'Scoping', 'Decision'], deals: [
    { company: 'Forma', name: 'Second team rollout', amount: '$9,000', initials: 'F', date: 'Oct 28', note: 'Clarify the second team’s workflow.', evidence: 'A customer reply asked whether another department could use the same workspace.', source: 'Team rollout · Support', stage: 0 },
    { company: 'Northstar Labs', name: 'Additional team seats', amount: '$12,000', initials: 'NL', date: 'Nov 06', note: 'Confirm scope after the admin pilot.', evidence: 'Maya asked to bring in the wider team once the SSO pilot is approved.', source: 'SSO rollout review · Meeting', stage: 1 },
    { company: 'Acme Studio', name: 'Product team workspace', amount: '$6,000', initials: 'AS', date: 'Nov 12', note: 'Review the proposed rollout schedule.', evidence: 'The product lead has reviewed the workspace and requested a phased rollout.', source: 'Expansion planning · Email', stage: 2 },
  ] },
];

export function CRMPipeline() {
  const [pipeline, setPipeline] = useState(0);
  const [selected, setSelected] = useState(1);
  const current = PIPELINES[pipeline];
  const deal = current.deals[selected];
  return <div className="crm-pipeline-demo">
    <div className="crm-pipeline-top"><div><b className="crm-workspace-mark">O</b><strong>Deals</strong><span>Illustrative pipeline</span></div><div className="crm-pipeline-switch" role="group" aria-label="Choose an example pipeline">{PIPELINES.map((item, i) => <button key={item.label} type="button" aria-pressed={pipeline === i} onClick={() => { setPipeline(i); setSelected(1); }}>{item.label}</button>)}</div></div>
    <div className="crm-board">{current.stages.map((stage, i) => { const item = current.deals[i]; return <div className="crm-board-column" key={stage}><div className="crm-stage-heading"><span><i />{stage}</span><small>1</small></div><button type="button" className="crm-deal-card" aria-pressed={selected === i} aria-controls="crm-selected-deal" onClick={() => setSelected(i)}><span className="crm-deal-company"><b>{item.initials}</b>{item.company}</span><strong>{item.name}</strong><span className="crm-deal-amount">{item.amount}<small>Annual value</small></span><span className="crm-deal-bottom"><span className="crm-avatar">SR</span>Sam Rivera <span><Clock3 size={11} />{item.date}</span></span></button></div>; })}</div>
    <div id="crm-selected-deal" className="crm-deal-context" role="region" aria-label={`Context for ${deal.name}`}><div><span className="crm-mini-label"><FileText size={13} /> THE CONTEXT BEHIND THE DEAL</span><h3>{deal.company} · {deal.name}</h3><p>{deal.evidence}</p><small>{deal.source}</small></div><div className="crm-deal-action"><span className="crm-mini-label">NEXT STEP</span><p>{deal.note}</p><span><span className="crm-avatar">SR</span>Sam Rivera <span>· Owner</span></span></div></div>
  </div>;
}

const SIGNALS = [
  { label: 'Buying intent', Icon: Sparkles, title: 'An opportunity inside a support reply.', quote: 'We can roll this out to the rest of the team once security approves SSO. Can we start with an admin-only pilot?', source: 'Maya Chen · Support conversation', context: 'Northstar Labs is evaluating a wider rollout. Security approval is the next milestone.', action: 'Review the pilot requirements and share the setup guide.', tag: 'Conversion', status: 'Needs review' },
  { label: 'Renewal risk', Icon: ShieldCheck, title: 'A concern to resolve before renewal.', quote: 'Before we renew, we need to know how the remaining SSO issue will be handled.', source: 'Maya Chen · Renewal email', context: 'A renewal discussion is waiting on a product issue. The account owner needs a clear update from the team.', action: 'Check the linked work and prepare an update for Maya.', tag: 'Renewal', status: 'Follow-up needed' },
  { label: 'Expansion', Icon: Building2, title: 'Another team is ready to join.', quote: 'Once the pilot is approved, we’d like to bring our customer success team into the workspace too.', source: 'Maya Chen · Rollout meeting', context: 'The customer is asking about a broader rollout, with approval of the current pilot still outstanding.', action: 'Confirm the team size and rollout requirements.', tag: 'Expansion', status: 'Needs review' },
];

export function CRMSignals() {
  const [selected, setSelected] = useState(0);
  const item = SIGNALS[selected];
  return <div className="crm-signals-demo"><div className="crm-signal-choices"><span className="crm-mini-label">EXPLORE AN EXAMPLE</span>{SIGNALS.map(({ label, Icon }, i) => <button key={label} type="button" aria-pressed={selected === i} aria-controls="crm-signal-evidence" onClick={() => setSelected(i)}><Icon size={18} /><span>{label}</span><ArrowRight size={16} /></button>)}<p>See the evidence.<br />Choose what happens next.</p></div><div id="crm-signal-evidence" className="crm-signal-evidence" role="region" aria-label={item.label + ' example'}><div className="crm-signal-meta"><span>{item.tag}</span><span><CircleDot size={11} />{item.status}</span></div><h3>{item.title}</h3><blockquote>“{item.quote}”</blockquote><div className="crm-signal-source"><span className="crm-avatar">MC</span><span>{item.source}</span></div><div className="crm-signal-reason"><span className="crm-mini-label">WHY IT MATTERS</span><p>{item.context}</p></div><div className="crm-signal-action"><Sparkles size={19} /><div><span className="crm-mini-label">PROPOSED NEXT STEP</span><p>{item.action}</p></div></div></div></div>;
}

export function CRMPlaybook() {
  return <MotionFrame name="Playbook"><div className="crm-playbook-demo"><div className="crm-playbook-heading"><span className="crm-mini-label">NORTHSTAR LABS / SALES HANDOFF</span><h3>Make the pilot ready to start.</h3><p>A shared outcome. Clear responsibilities.</p></div><Step index={0} className="crm-milestone"><span className="crm-milestone-icon"><Check size={15} /></span><div><strong>Confirm the pilot requirements</strong><small>Customer context reviewed</small></div><span className="crm-status">Complete</span></Step><Step index={1} className="crm-milestone"><span className="crm-milestone-icon"><CircleDot size={15} /></span><div><strong>Validate group-to-role mapping</strong><small>Linked task · Sam Rivera</small></div><span className="crm-status crm-status-amber">In progress</span></Step><Step index={2} className="crm-milestone"><span className="crm-milestone-icon"><Circle size={15} /></span><div><strong>Confirm the handoff</strong><small>After the security review</small></div></Step><Step index={3} className="crm-playbook-proposal"><div><Sparkles size={15} /><span>Agent proposal</span><span className="crm-review-pill">Approval required</span></div><p>Prepare a follow-up with the setup guide and outstanding security checks.</p><span><Mail size={13} /> Customer message · Draft for review</span></Step><div className="crm-playbook-foot"><CheckCheck size={15} /> Milestones, work, and decisions stay connected.</div></div></MotionFrame>;
}
