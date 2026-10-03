import { useEffect, useRef } from 'react';
import { ArrowLeft, ArrowUpRight, Bot, Check, Clock3, FileText, GitPullRequest, Search, ShieldCheck, TrendingUp } from 'lucide-react';

// Prepared OrbitDesk examples from server/internal/templates/manifests:
// triage_failing_checks, release_notes_writer, docs_freshness_sweep,
// buying_signal_to_task. Approval rules illustrate a team's chosen setup.
const TEMPLATES = [
  {
    key: 'triage_failing_checks', name: 'Triage failing checks', category: 'Engineering', Icon: GitPullRequest,
    description: 'Find the cause of a failed check and save the diagnosis.',
    agent: 'Check triage agent', trigger: 'GitHub check fails', scope: 'orbitdesk/app · main',
    context: ['Check logs', 'Repository files'],
    action: 'Save a diagnosis report', result: 'Engineering docs · Findings and suggested fixes',
    approval: 'Approve follow-up tasks', review: 'Review proposed tasks before they are created.',
  },
  {
    key: 'release_notes_writer', name: 'Release notes writer', category: 'Docs', Icon: FileText,
    description: 'Draft release notes from the changes your team shipped.',
    agent: 'Release notes agent', trigger: 'GitHub release published', scope: 'orbitdesk/app · v2.8',
    context: ['Release details', 'Linked tasks', 'Existing docs'],
    action: 'Prepare the release notes', result: 'Internal docs · Release notes proposal',
    approval: 'Review before publishing', review: 'Your team checks the draft before customers see it.',
  },
  {
    key: 'docs_freshness_sweep', name: 'Internal docs freshness sweep', category: 'Docs', Icon: Search,
    description: 'Find outdated instructions before your team relies on them.',
    agent: 'Quill', trigger: 'Every Monday at 9:00', scope: 'Internal docs · Product team',
    context: ['Internal docs', 'Connected repository'],
    action: 'Check the docs against the product', result: 'Docs report · Outdated guidance and suggested changes',
    approval: 'Review the recommended changes', review: 'Your team decides which updates to apply.',
  },
  {
    key: 'buying_signal_to_task', name: 'High-intent buyer signal to task', category: 'Sales', Icon: TrendingUp,
    description: 'Turn clear buying signals into follow-up work for your team.',
    agent: 'Sales follow-up agent', trigger: 'Every weekday at 9:00', scope: 'CRM · New high-confidence signals',
    context: ['Customer evidence', 'CRM signals', 'Existing tasks'],
    action: 'Prepare a sales follow-up task', result: 'Sales team · Next step with the customer evidence attached',
    approval: 'Approve the next step', review: 'Review the proposed task before it is created.',
  },
];

export function FlowTemplatesPreview({ selected, onSelect, onBack, id }: {
  selected: number | null; onSelect: (index: number) => void; onBack: () => void; id: string;
}) {
  const title = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (selected !== null) title.current?.focus({ preventScroll: true });
  }, [selected]);

  if (selected === null) return (
    <ul className="adp-flow-templates">
      {TEMPLATES.map(({ key, name, category, Icon, description, agent }, index) => (
        <li key={key}>
          <button className="adp-flow-card" id={`${id}-flow-${index}`} type="button" onClick={() => onSelect(index)} aria-label={`Preview ${name}`}>
            <span className="adp-flow-card-top"><i><Icon size={18} /></i><small>{category}</small><ArrowUpRight size={15} /></span>
            <strong>{name}</strong>
            <span className="adp-flow-description">{description}</span>
            <span className="adp-flow-owner"><Bot size={14} />{agent}<span>View flow →</span></span>
          </button>
        </li>
      ))}
    </ul>
  );

  const flow = TEMPLATES[selected];
  return (
    <div className="adp-flow-detail">
      <button className="adp-flow-back" type="button" onClick={onBack} aria-label="Back to flow templates"><ArrowLeft size={15} />All templates</button>
      <h4 tabIndex={-1} ref={title}>{flow.name}</h4>
      <ol className="adp-flow-steps">
        <li className="adp-flow-step">
          <i><Clock3 size={17} /></i>
          <div><small>When</small><strong>{flow.trigger}</strong><p>{flow.scope}</p></div>
        </li>
        <li className="adp-flow-step">
          <i><Search size={17} /></i>
          <div><small>Context</small><strong>Read the relevant information</strong><div className="adp-flow-context">{flow.context.map(source => <span key={source}><Check size={11} />{source}</span>)}</div></div>
        </li>
        <li className="adp-flow-step">
          <i><Bot size={17} /></i>
          <div><small>{flow.agent}</small><strong>{flow.action}</strong><p>{flow.result}</p></div>
        </li>
        <li className="adp-flow-step adp-flow-approval">
          <i><ShieldCheck size={17} /></i>
          <div><small>Team review</small><strong>{flow.approval}</strong><p>{flow.review}</p></div>
        </li>
      </ol>
    </div>
  );
}
