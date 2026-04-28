import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { BotIcon, Loading01Icon, Settings02Icon, Tick01Icon } from '@/lib/icons';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { commandBarService } from '@/lib/services/commandBarService';
import { cn } from '@/lib/utils';
import type { AgentRun, CommandBarPlanStep, CommandBarToolCatalogResponse } from '@/lib/pmTypes';

interface PromotionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string | undefined;
  run: AgentRun | null;
  step: CommandBarPlanStep | null;
  planPrompt?: string;
  onPromoted?: (agentName: string) => void;
}

const TARGET_LABELS: Record<string, string> = {
  task: 'Task',
  epic: 'Epic',
  document: 'Document',
  crm_contact: 'CRM Contact',
  crm_deal: 'CRM Deal',
  workspace: 'Workspace',
};

function suggestAgentName(step: CommandBarPlanStep | null, prompt?: string): string {
  if (!step) return '';
  const base = step.agent_name?.trim() || 'Agent';
  const verb = (prompt ?? step.instructions ?? '')
    .trim()
    .replace(/^(please\s+|can\s+you\s+|could\s+you\s+|i\s+want\s+to\s+|i\s+need\s+to\s+)/i, '')
    .split(/\s+/)
    .slice(0, 4)
    .join(' ')
    .replace(/[.!?,:;]+$/g, '');
  if (!verb) return base;
  const titled = verb
    .split(' ')
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1).toLowerCase())
    .join(' ');
  return `${base} — ${titled}`.slice(0, 80);
}

export function PromotionDialog({
  open,
  onOpenChange,
  workspaceId,
  run,
  step,
  planPrompt,
  onPromoted,
}: PromotionDialogProps) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [catalog, setCatalog] = useState<CommandBarToolCatalogResponse | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(false);
  /** Selected tools. undefined = inherit from source run (step.allowed_tools or all). */
  const [selectedTools, setSelectedTools] = useState<string[] | undefined>(undefined);
  /** Selected targets. undefined = inherit (source run target only). */
  const [selectedTargets, setSelectedTargets] = useState<string[] | undefined>(undefined);

  const targetTitle =
    run?.target_info?.title ||
    run?.target_info?.task_key ||
    step?.target.display_title ||
    run?.target_id?.slice(0, 8) ||
    '—';
  const targetTypeLabel = TARGET_LABELS[run?.target_type ?? step?.target.entity_type ?? ''] ?? run?.target_type ?? '';

  const sourceTargets = catalog?.allowed_targets ?? [];
  const targetableOptions = useMemo(() => {
    const set = new Set<string>(sourceTargets);
    if (run?.target_type) set.add(run.target_type);
    return Array.from(set);
  }, [run?.target_type, sourceTargets]);

  const inheritedTools = useMemo<string[]>(
    () => (step?.allowed_tools && step.allowed_tools.length > 0 ? step.allowed_tools : (catalog?.allowed_tools ?? [])),
    [catalog, step],
  );

  // Reset on open
  useEffect(() => {
    if (open) {
      setName(suggestAgentName(step, planPrompt));
      setDescription('');
      setSubmitting(false);
      setSelectedTools(undefined);
      setSelectedTargets(undefined);
      setCatalog(null);
      setCatalogLoading(false);
    }
  }, [open, step, planPrompt]);

  // Fetch the agent tool catalog when dialog opens (so we can show labels and the source agent's full target allowlist).
  useEffect(() => {
    if (!open || !workspaceId || !run?.agent_id) return;
    let cancelled = false;
    setCatalogLoading(true);
    void commandBarService
      .getAgentToolCatalog(workspaceId, run.agent_id, step?.allowed_tools)
      .then((res) => {
        if (cancelled) return;
        if (res.data) setCatalog(res.data);
      })
      .finally(() => {
        if (!cancelled) setCatalogLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, run?.agent_id, step?.allowed_tools, workspaceId]);

  const effectiveSelectedTools = selectedTools ?? inheritedTools;
  const effectiveSelectedTargets = selectedTargets ?? [run?.target_type ?? 'workspace'];
  const toolsNarrowed = selectedTools !== undefined && selectedTools.length !== inheritedTools.length;
  const noToolSelected = selectedTools !== undefined && selectedTools.length === 0;
  const targetsNarrowed = selectedTargets !== undefined;
  const noTargetSelected = selectedTargets !== undefined && selectedTargets.length === 0;

  const toggleTool = (tool: string) => {
    const next = new Set(effectiveSelectedTools);
    if (next.has(tool)) next.delete(tool);
    else next.add(tool);
    if (next.size === inheritedTools.length && [...next].every((t) => inheritedTools.includes(t))) {
      setSelectedTools(undefined);
    } else {
      setSelectedTools(Array.from(next));
    }
  };
  const toggleTarget = (target: string) => {
    const next = new Set(effectiveSelectedTargets);
    if (next.has(target)) next.delete(target);
    else next.add(target);
    setSelectedTargets(Array.from(next));
  };

  const canSubmit =
    !!workspaceId &&
    !!run &&
    !!step &&
    name.trim().length >= 2 &&
    !submitting &&
    !noToolSelected &&
    !noTargetSelected;

  const submit = async () => {
    if (!canSubmit || !workspaceId || !run) return;
    setSubmitting(true);
    try {
      const res = await commandBarService.promoteRunToAgent(workspaceId, run.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        allowed_tools: selectedTools,
        allowed_targets: selectedTargets,
      });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to save agent');
        return;
      }
      toast.success(`Saved ${res.data.agent.name}`);
      onPromoted?.(res.data.agent.name);
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  };

  if (!run || !step) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <BotIcon className="h-4 w-4 text-muted-foreground" />
            Save as reusable agent
          </DialogTitle>
          <DialogDescription>
            Reuse this run's agent + instructions next time. Narrow the tools or targets if you want a tighter scope than the source agent.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="promotion-name">Name</Label>
            <Input
              id="promotion-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Doc Refresher"
              autoFocus
              maxLength={120}
            />
            <p className="text-[11px] text-muted-foreground">Pick a name your team will recognize. You can edit it later.</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="promotion-description">Description (optional)</Label>
            <Textarea
              id="promotion-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What is this agent for? When should someone reach for it?"
              rows={2}
              maxLength={500}
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label>Targets</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-[11px]"
                onClick={() => setSelectedTargets(undefined)}
                disabled={!targetsNarrowed}
              >
                Reset
              </Button>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {targetableOptions.map((target) => {
                const checked = effectiveSelectedTargets.includes(target);
                return (
                  <button
                    type="button"
                    key={target}
                    onClick={() => toggleTarget(target)}
                    className={cn(
                      'rounded-full border px-3 py-1 text-xs transition',
                      checked
                        ? 'border-primary bg-primary text-primary-foreground'
                        : 'border-border/70 text-muted-foreground hover:border-border hover:text-foreground',
                    )}
                  >
                    {TARGET_LABELS[target] ?? target}
                  </button>
                );
              })}
            </div>
            <p className={cn('text-[11px] text-muted-foreground', noTargetSelected && 'text-destructive')}>
              {noTargetSelected
                ? 'Pick at least one target.'
                : targetsNarrowed
                  ? `Narrowed to ${effectiveSelectedTargets.length} target${effectiveSelectedTargets.length === 1 ? '' : 's'}.`
                  : `Inherits from this run: ${TARGET_LABELS[run.target_type] ?? run.target_type}.`}
            </p>
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label>Tools</Label>
              {toolsNarrowed ? (
                <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={() => setSelectedTools(undefined)}>
                  Reset to inherited
                </Button>
              ) : null}
            </div>
            {catalogLoading ? (
              <div className="flex items-center gap-2 rounded border border-border/60 px-3 py-3 text-xs text-muted-foreground">
                <Loading01Icon className="h-3 w-3 animate-spin" />
                Loading tool catalog...
              </div>
            ) : catalog && catalog.tools.length > 0 ? (
              <div className="max-h-44 space-y-1 overflow-y-auto rounded border border-border/60 px-2 py-1.5">
                {catalog.tools
                  .filter((tool) => tool.allowed)
                  .map((tool) => {
                    const checked = effectiveSelectedTools.includes(tool.name);
                    return (
                      <label
                        key={tool.name}
                        className="flex cursor-pointer items-start gap-2 rounded px-1.5 py-1 hover:bg-muted/40"
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={() => toggleTool(tool.name)}
                          className="mt-0.5"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1.5">
                            <span className="truncate text-xs font-medium">{tool.name.replace(/[_:]/g, ' ').replace(/\b\w/g, (m) => m.toUpperCase())}</span>
                            {tool.category ? (
                              <Badge variant="outline" className="text-[9px] uppercase tracking-wider">{tool.category}</Badge>
                            ) : null}
                          </div>
                          {tool.description ? (
                            <p className="line-clamp-2 text-[11px] text-muted-foreground">{tool.description}</p>
                          ) : null}
                        </div>
                      </label>
                    );
                  })}
              </div>
            ) : (
              <p className="rounded border border-border/60 px-3 py-2 text-[11px] text-muted-foreground">
                Inherits the base agent's full tool allowlist.
              </p>
            )}
            <p className={cn('text-[11px] text-muted-foreground', noToolSelected && 'text-destructive')}>
              {noToolSelected
                ? 'Pick at least one tool.'
                : toolsNarrowed
                  ? `${effectiveSelectedTools.length} of ${inheritedTools.length} selected.`
                  : `Inherits ${inheritedTools.length} tool${inheritedTools.length === 1 ? '' : 's'} from the source run.`}
            </p>
          </div>

          <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2.5 space-y-1">
            <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              <Settings02Icon className="h-3 w-3" />
              Provenance
            </div>
            <dl className="grid grid-cols-[auto,1fr] gap-x-3 gap-y-1 text-xs">
              <dt className="text-muted-foreground">Base agent</dt>
              <dd className="truncate font-medium">{step.agent_name}</dd>

              <dt className="text-muted-foreground">Source target</dt>
              <dd className="truncate">
                <Badge variant="outline" className="mr-1.5 text-[10px]">{targetTypeLabel}</Badge>
                <span className="text-foreground">{targetTitle}</span>
              </dd>

              {step.instructions ? (
                <>
                  <dt className="self-start text-muted-foreground">Instruction</dt>
                  <dd className="text-foreground/90 line-clamp-2">{step.instructions}</dd>
                </>
              ) : null}
            </dl>
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={!canSubmit}>
            {submitting ? <Loading01Icon className="mr-1.5 h-4 w-4 animate-spin" /> : <Tick01Icon className="mr-1.5 h-4 w-4" />}
            Save agent
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
