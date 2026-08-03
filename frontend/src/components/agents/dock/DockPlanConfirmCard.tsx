import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import type { DockPlanConfirmPayload, DockPlanConfirmStep } from '@/lib/dockTypes';

interface DockPlanConfirmCardProps {
  payload: DockPlanConfirmPayload;
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
export function DockPlanConfirmCard({ payload, onDecision }: DockPlanConfirmCardProps) {
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

  const action = payload.action ?? {};
  const steps = action.steps ?? [];

  return (
    <div className="rounded-lg border border-amber-300/60 bg-amber-50/60 p-3 text-sm dark:border-amber-500/30 dark:bg-amber-500/10">
      <div className="mb-2 font-medium text-foreground">
        {payload.summary?.trim() || payload.title?.trim() || 'The agent wants to run this — confirm?'}
      </div>
      {steps.length > 0 && (
        <ol className="mb-2 space-y-1.5">
          {steps.map((step, index) => (
            <li key={index} className="rounded-md border border-border/60 bg-background/80 px-2.5 py-1.5">
              <DockPlanConfirmStepRow step={step} index={index} />
            </li>
          ))}
        </ol>
      )}
      {action.description && (
        <p className="mb-2 text-muted-foreground">
          New agent{action.name ? ` “${action.name}”` : ''}: {action.description}
        </p>
      )}
      {action.run_id && (
        <p className="mb-2 text-muted-foreground">
          Promote run <span className="font-mono text-xs">{action.run_id}</span>
          {action.name ? ` to saved agent “${action.name}”` : ''}
        </p>
      )}
      {rejecting ? (
        <div className="space-y-2">
          <textarea
            autoFocus
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="What should the agent change? (optional)"
            className="w-full resize-none rounded-md border border-border bg-background px-2 py-1.5 text-sm"
            rows={2}
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
        <div className="flex items-center gap-2">
          <Button size="sm" disabled={acting !== null} onClick={() => void decide('approve')}>
            {acting === 'approve' ? 'Approving…' : 'Approve'}
          </Button>
          <Button size="sm" variant="outline" disabled={acting !== null} onClick={() => setRejecting(true)}>
            Not now
          </Button>
        </div>
      )}
    </div>
  );
}

function DockPlanConfirmStepRow({ step, index }: { step: DockPlanConfirmStep; index: number }) {
  const agentLabel = step.use_command_agent ? 'Command Agent (one-shot)' : step.agent_id || 'agent';
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
