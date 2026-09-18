import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { FlowSemanticConditionField } from '@/components/automation/FlowSemanticConditionField';
import { ActivityTableRow } from '@/pages/automation/AutomationActivity';
import { draftFromRule, serializeDraft, validateDraft } from '@/pages/automation/AutomationFlows';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { AutomationRule } from '@/lib/pmTypes';
import '@/index.css';

const rule: AutomationRule = { id: 'rule', workspace_id: 'workspace', name: 'Authentication release', enabled: true, trigger_type: 'github.release_published', trigger_config: { repo_full_name: 'helpin/api', semantic_condition: { text: 'The release concerns authentication' } }, action_type: 'start_agent_run', action_config: { agent_id: 'agent', target_type: 'workspace', target_id: 'workspace' }, position: 0, stop_on_match: false, created_at: '', updated_at: '' };
function Harness() {
 const missingKey = new URLSearchParams(window.location.search).has('missingKey');
 const [draft, setDraft] = useState(() => draftFromRule(rule, [], 'UTC'));
 const [saved, setSaved] = useState('');
 const [error, setError] = useState('');
 return <TooltipProvider><main className="mx-auto max-w-2xl space-y-4 p-6">
  <h1>Flow condition</h1>
  <button onClick={() => setDraft({ ...draft, triggerType: draft.triggerType === 'cron' ? 'github.release_published' : 'cron' })}>Switch trigger</button>
  <FlowSemanticConditionField availability={missingKey ? { available: false, reason: 'not_configured' } : { available: true }} value={draft.semanticCondition} scheduled={draft.triggerType === 'cron'} onChange={(semanticCondition) => setDraft({ ...draft, semanticCondition })} />
  <button onClick={() => { const message = validateDraft(draft); setError(message || ''); if (!message) setSaved(JSON.stringify(serializeDraft(draft, 'workspace', 'UTC').trigger_config)); }}>Save</button>
  <p role="alert">{error}</p><output className="block break-words">{saved}</output>
  <ActivityTableRow item={{ execution_id: 'execution', agent_id: '', agent_name: '', binding_id: 'semantic_condition', binding_kind: 'automation_rule', binding_title: rule.name, reference_id: rule.id, reference_type: 'automation_rule', reference_title: rule.name, condition_outcome: 'uncertain', condition_assessment_id: 'assessment', status: 'skipped', fired_at: '2026-09-17T10:00:00Z' }} onOpenRun={() => {}} onOpenFlow={() => {}} onOpenTarget={() => {}} />
 </main></TooltipProvider>;
}
createRoot(document.getElementById('root')!).render(<Harness />);
