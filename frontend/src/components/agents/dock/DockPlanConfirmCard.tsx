import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { QuietUnderlineTextarea } from '@/components/design-system/quiet';
import type { DockPlanConfirmPayload, DockPlanConfirmStep } from '@/lib/dockTypes';
import { useAIProfiles } from '@/hooks/queries/useAIProfiles';

interface DockPlanConfirmCardProps {
  payload: DockPlanConfirmPayload;
  workspaceId?: string;
  onDecision: (decision: 'approve' | 'request_changes', note?: string) => Promise<{ error: string | null }>;
}

/**
 * Confirm card for dock launches: renders the agent's proposed action
 * (child-run steps, agent creation, or run promotion) from a
 * dock_plan_confirm approval payload and resolves it with approve /
 * request-changes. The server independently verifies that the launched action
 * matches the approved payload, so this card is the human gate, not the
 * enforcement point.
 */
export function DockPlanConfirmCard({ payload, onDecision, workspaceId = '' }: DockPlanConfirmCardProps) {
  const [acting, setActing] = useState<'approve' | 'request_changes' | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [note, setNote] = useState('');

  const decide = async (decision: 'approve' | 'request_changes', decisionNote?: string) => {
    setActing(decision);
    try {
      const res = await onDecision(decision, decisionNote?.trim() || undefined);
      if (res.error) toast.error(res.error);
      else setRejecting(false);
    } finally {
      setActing(null);
    }
  };

  const action = payload.action ?? payload.raw_input?.action ?? {};
  const profiles = useAIProfiles(workspaceId, { enabled: !!workspaceId && !!action.epic_id });
  const approvedProfile = profiles.data?.find((profile) => profile.id === action.ai_profile_id);
  const steps = action.steps ?? [];

  return (
    <section
      aria-label={payload.title?.trim() || 'Confirm agent action'}
      className="border-y border-amber-400/40 py-3 text-sm dark:border-amber-500/35"
      data-agent-dock-plan-confirm
    >
      <div className="mb-1 text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-400">
        Needs your approval
      </div>
      <div className="font-semibold text-foreground">
        {payload.summary?.trim() || payload.title?.trim() || 'The agent wants to run this — confirm?'}
      </div>
      {action.epic_id ? <div className="mt-3 border-t border-border/60 pt-3 text-xs text-muted-foreground">
        <div>Epic <span className="font-mono text-foreground">{action.epic_id}</span></div>
        <div className="mt-1">AI model: <span className="text-foreground">{action.ai_profile_id ? approvedProfile ? `${approvedProfile.name}${approvedProfile.scope === 'personal' ? ' (personal)' : ' (shared)'}` : action.ai_profile_id : 'Agent defaults'}</span></div>
        <div className="mt-1">To change the model, request changes before approval.</div>
      </div> : null}
      {steps.length > 0 && (
        <ol className="mt-3 divide-y divide-border/60 border-y border-border/60" data-agent-dock-plan-steps>
          {steps.map((step, index) => (
            <li key={index} className="py-2.5">
              <DockPlanConfirmStepRow step={step} index={index} />
            </li>
          ))}
        </ol>
      )}
      {(action.operations?.length ?? 0) > 0 && (
        <div className="mt-3 divide-y divide-border/60 border-y border-border/60" data-agent-dock-plan-operations>
          {action.operations!.map((operation, index) => (
            <div key={`${operation.tool_name}-${index}`} className="py-2.5">
              <div className="flex items-center gap-2 text-xs">
                <span className="font-mono font-medium text-foreground">{operation.tool_name}</span>
                <span className="text-muted-foreground">up to {operation.max_calls ?? 1} call{(operation.max_calls ?? 1) === 1 ? '' : 's'}</span>
              </div>
              {operation.constraints && Object.keys(operation.constraints).length > 0 && (
                <div className="mt-1 text-xs text-muted-foreground">
                  {Object.entries(operation.constraints).map(([key, value]) => `${key}: ${String(value)}`).join(' · ')}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
      {(action.expected_outcomes?.length ?? 0) > 0 && (
        <ul className="mt-3 list-disc space-y-0.5 border-t border-border/60 pt-3 pl-5 text-xs text-muted-foreground">
          {action.expected_outcomes!.map((outcome, index) => <li key={index}>{outcome}</li>)}
        </ul>
      )}
      {action.proposal_id && (action.operations?.length ?? 0) === 0 && steps.length === 0 && (
        <p className="mt-3 border-t border-border/60 pt-3 text-xs text-muted-foreground">Scoped execution proposal <span className="font-mono">{action.proposal_id.slice(0, 8)}</span></p>
      )}
      {action.description && (
        <p className="mt-3 border-t border-border/60 pt-3 text-muted-foreground">
          New agent{action.name ? ` “${action.name}”` : ''}: {action.description}
        </p>
      )}
      {action.run_id && (
        <p className="mt-3 border-t border-border/60 pt-3 text-muted-foreground">
          Promote run <span className="font-mono text-xs">{action.run_id}</span>
          {action.name ? ` to saved agent “${action.name}”` : ''}
        </p>
      )}
      {rejecting ? (
        <div className="mt-3 space-y-2 border-t border-border/60 pt-3">
          <QuietUnderlineTextarea
            autoFocus
            aria-label="Optional change request"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="What should change? (optional)"
          />
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={acting !== null}
              onClick={() => void decide('request_changes', note)}
            >
              {acting === 'request_changes' ? 'Sending…' : 'Send'}
            </Button>
            <Button size="sm" variant="ghost" disabled={acting !== null} onClick={() => setRejecting(false)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <div className="mt-3 flex items-center gap-2 border-t border-border/60 pt-3">
          <Button size="sm" disabled={acting !== null} onClick={() => void decide('approve')}>
            {acting === 'approve' ? 'Approving…' : 'Approve'}
          </Button>
          <Button size="sm" variant="outline" disabled={acting !== null} onClick={() => setRejecting(true)}>
            Not now
          </Button>
        </div>
      )}
    </section>
  );
}

function DockPlanConfirmStepRow({ step, index }: { step: DockPlanConfirmStep; index: number }) {
  const agentLabel = step.use_command_agent ? 'Sub-agent' : step.agent_id || 'agent';
  const targetLabel =
    step.target?.type && step.target.type !== 'workspace'
      ? `${step.target.type}${step.target.id ? ` ${step.target.id.slice(0, 8)}` : ''}`
      : 'workspace';
  return (
    <div>
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="font-mono">{index + 1}.</span>
        <span className="font-medium text-foreground">{agentLabel}</span>
        <span>on {targetLabel}</span>
        {(step.depends_on_step_indexes?.length ?? 0) > 0 && (
          <span>after step{step.depends_on_step_indexes!.length > 1 ? 's' : ''} {step.depends_on_step_indexes!.map((i) => i + 1).join(', ')}</span>
        )}
      </div>
      {step.instructions && <div className="mt-0.5 text-sm">{step.instructions}</div>}
      {(step.allowed_tools?.length ?? 0) > 0 && (
        <div className="mt-1 flex flex-wrap gap-1">
          {step.allowed_tools!.map((tool) => (
            <span key={tool} className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
              {tool}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
