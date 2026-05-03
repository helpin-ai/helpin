import { useMemo } from 'react';
import {
  BotIcon,
  Loading01Icon,
  PencilEdit01Icon,
  Target01Icon,
  Tick01Icon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import type { CommandBarParseResponse, CommandBarPlan, CommandBarPlanStep } from '@/lib/pmTypes';
import {
  buildTaskNodes,
  hasAnyDependencies,
  layerTasks,
  taskNounFor,
  type PlanLayer,
  type TaskNode,
} from './planLayers';
import { describeStep } from './utils';

type Plan = NonNullable<Extract<CommandBarParseResponse, { status: 'plan' }>['plan']>;

interface PlanPreviewProps {
  plan: Plan;
  rationale: string | null;
  dispatching: boolean;
  onConfirm: () => void;
  onEdit: () => void;
  onDiscard: () => void;
}

export function PlanPreview({
  plan,
  rationale,
  dispatching,
  onConfirm,
  onEdit,
  onDiscard,
}: PlanPreviewProps) {
  const nodes = useMemo(() => buildTaskNodes(plan), [plan]);
  const layers = useMemo(() => layerTasks(nodes), [nodes]);
  const layered = layers.length > 1 || (layers.length === 1 && hasAnyDependencies(plan));
  const taskCount = nodes.length;
  const stepCount = plan.steps.length;
  const noun = taskNounFor(nodes);
  const showRationale = !!rationale && plan.plan_kind !== 'one_shot_command';

  const runCount = plan.estimated_runs ?? plan.run_count;

  return (
    <div>
      {plan.steps.length === 1 ? (
        <SingleStepCard step={plan.steps[0]} />
      ) : (
        <>
          <PlanHeader
            taskCount={taskCount}
            stepCount={stepCount}
            nounPlural={noun.plural}
            nounSingular={noun.singular}
          />
          {showRationale ? (
            <p className="mb-2 line-clamp-3 text-xs text-muted-foreground">{rationale}</p>
          ) : null}
          {plan.guardrails?.length ? <GuardrailList guardrails={plan.guardrails} /> : null}
          {layered ? (
            <LayeredBody plan={plan} layers={layers} noun={noun} />
          ) : (
            <FlatBody plan={plan} nodes={nodes} />
          )}
        </>
      )}
      {plan.steps.length === 1 && plan.guardrails?.length ? (
        <GuardrailList guardrails={plan.guardrails} />
      ) : null}
      <ActionRow
        runCount={runCount}
        dispatching={dispatching}
        onConfirm={onConfirm}
        onEdit={onEdit}
        onDiscard={onDiscard}
      />
    </div>
  );
}

function PlanHeader({
  taskCount,
  stepCount,
  nounPlural,
  nounSingular,
}: {
  taskCount: number;
  stepCount: number;
  nounPlural: string;
  nounSingular: string;
}) {
  // TODO: render `· ~Xm` when backend exposes estimated_duration_seconds.
  const noun = taskCount === 1 ? nounSingular : nounPlural;
  return (
    <div className="mb-2 flex items-baseline gap-2">
      <span className="text-sm font-semibold text-foreground">Plan</span>
      <span className="text-[11px] text-muted-foreground">
        {taskCount} {noun} · {stepCount} step{stepCount === 1 ? '' : 's'}
      </span>
    </div>
  );
}

function GuardrailList({
  guardrails,
}: {
  guardrails: NonNullable<CommandBarPlan['guardrails']>;
}) {
  return (
    <div className="mb-2 space-y-1">
      {guardrails.map((g, i) => (
        <div key={i} className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Target01Icon className="h-3 w-3" />
          <span>{g.message}</span>
        </div>
      ))}
    </div>
  );
}

function SingleStepCard({ step }: { step: CommandBarPlanStep }) {
  const desc = describeStep(step.instructions);
  return (
    <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
      <div className="flex items-center gap-2">
        <BotIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate text-sm font-medium text-foreground">{step.agent_name}</span>
        {step.target?.display_title ? (
          <>
            <span className="opacity-60">·</span>
            <span className="truncate text-[11px] text-muted-foreground">
              {step.target.display_title}
            </span>
          </>
        ) : null}
      </div>
      {desc ? (
        <p className="mt-1 line-clamp-3 pl-[22px] text-xs leading-snug text-muted-foreground">
          {desc}
        </p>
      ) : null}
    </div>
  );
}

function LayeredBody({
  plan,
  layers,
  noun,
}: {
  plan: Plan;
  layers: PlanLayer[];
  noun: { singular: string; plural: string };
}) {
  return (
    <div className="relative">
      {/* Vertical rail runs the full body length. Dots in TaskRow / heading
          sit on this rail at x = 7px (rail width 1px, centered at 7.5px). */}
      <div aria-hidden className="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-border/70" />
      <div className="space-y-2.5">
        {layers.map((layer, i) => (
          <div key={layer.index} className="space-y-1">
            <LayerHeading
              layerIndex={i}
              totalLayers={layers.length}
              taskCount={layer.tasks.length}
              noun={noun}
            />
            <div className="space-y-0.5">
              {layer.tasks.map((task) => (
                <TaskRow key={task.key} plan={plan} task={task} fallbackIndex={undefined} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function LayerHeading({
  layerIndex,
  totalLayers,
  taskCount,
  noun,
}: {
  layerIndex: number;
  totalLayers: number;
  taskCount: number;
  noun: { singular: string; plural: string };
}) {
  const text = (() => {
    if (layerIndex === 0) {
      if (totalLayers > 1) return 'blocker';
      return 'starts now';
    }
    const word = taskCount === 1 ? noun.singular : noun.plural;
    return taskCount > 1
      ? `then ${taskCount} ${word} fan out in parallel`
      : `then ${word} runs`;
  })();
  return (
    <div className="flex items-center">
      {/* 14px slot keeps text aligned with TaskRow text (dot+stub take 14px). */}
      <span aria-hidden className="block w-[14px] shrink-0" />
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {text}
      </span>
    </div>
  );
}

function FlatBody({ plan, nodes }: { plan: Plan; nodes: TaskNode[] }) {
  if (nodes.length === 1 && plan.steps.length > 1) {
    // Pipeline against a single target — show the agent chain in one card.
    return (
      <div className="rounded-md border border-border/60 bg-muted/30 px-2.5 py-2">
        <div className="flex items-center gap-2">
          <span className="truncate text-[11px] text-muted-foreground">
            {displayTitle(nodes[0])}
          </span>
        </div>
        <div className="mt-1.5">
          <AgentChain plan={plan} stepIndexes={nodes[0].stepIndexes} />
        </div>
      </div>
    );
  }
  const noun = taskNounFor(nodes);
  const heading =
    nodes.length > 1
      ? `${nodes.length} ${nodes.length === 1 ? noun.singular : noun.plural} fan out in parallel`
      : null;
  return (
    <div className="relative">
      <div aria-hidden className="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-border/70" />
      {heading ? (
        <div className="mb-1 flex items-center gap-2 pl-3">
          <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            {heading}
          </span>
        </div>
      ) : null}
      <div className="space-y-0.5">
        {nodes.map((task, i) => (
          <TaskRow key={task.key} plan={plan} task={task} fallbackIndex={i + 1} />
        ))}
      </div>
    </div>
  );
}

function TaskRow({
  plan,
  task,
  fallbackIndex,
}: {
  plan: Plan;
  task: TaskNode;
  fallbackIndex?: number;
}) {
  const prefix = task.taskKey
    ? `#${shortTaskKey(task.taskKey)}`
    : fallbackIndex !== undefined
      ? `#${fallbackIndex}`
      : null;
  return (
    <div className="group flex items-center gap-2 py-1 pr-1">
      {/* Dot center sits at x=7 (matching the rail's left:7px). The dot is
          8px wide → its left starts at 3px, leaving a 1px stub from the
          rail to the dot edge. The connector after the dot is 4px wide. */}
      <span aria-hidden className="ml-[3px] inline-block h-2 w-2 shrink-0 rounded-full border border-border/70 bg-background" />
      <span aria-hidden className="inline-block h-px w-1 bg-border/70" />
      {prefix ? (
        <span className="shrink-0 rounded bg-muted px-1 py-0.5 text-[10px] font-mono text-muted-foreground">
          {prefix}
        </span>
      ) : null}
      <span className="min-w-0 flex-1 truncate text-xs font-medium text-foreground">
        {displayTitle(task)}
      </span>
      <AgentChain plan={plan} stepIndexes={task.stepIndexes} />
      {/* TODO: per-task duration estimate (e.g. ~2m) once backend provides it. */}
    </div>
  );
}

/**
 * Smooth over backend-supplied "Task <uuid>" placeholder titles. When the
 * planner can't resolve a real title it sometimes returns the entity_id
 * verbatim or a generic "Task <id>" string; show "Untitled task" instead so
 * the row reads as a row, not as a hex string.
 */
function displayTitle(task: TaskNode): string {
  const t = (task.title ?? '').trim();
  if (!t) return 'Untitled';
  if (t === task.key) return 'Untitled';
  if (/^(Task|Story|Epic|Doc|Document|Contact|Deal)\s+[0-9a-f-]{6,}$/i.test(t)) {
    return 'Untitled';
  }
  return t;
}

function shortTaskKey(taskKey: string): string {
  const m = /^[A-Z]+-(\d+)$/.exec(taskKey);
  return m ? m[1] : taskKey;
}

function AgentChain({ plan, stepIndexes }: { plan: Plan; stepIndexes: number[] }) {
  return (
    <div className="flex shrink-0 items-center gap-1">
      {stepIndexes.map((i, idx) => {
        const step = plan.steps[i];
        const initial = step.agent_name.trim().charAt(0).toUpperCase() || '·';
        return (
          <div key={i} className="flex items-center gap-1">
            <span
              title={step.agent_name}
              className="grid h-4 w-4 place-items-center rounded-full border border-border/70 bg-background text-[9px] font-semibold text-muted-foreground"
            >
              {initial}
            </span>
            {idx < stepIndexes.length - 1 ? (
              <span aria-hidden className="text-[10px] text-muted-foreground">
                →
              </span>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function ActionRow({
  runCount,
  dispatching,
  onConfirm,
  onEdit,
  onDiscard,
}: {
  runCount: number;
  dispatching: boolean;
  onConfirm: () => void;
  onEdit: () => void;
  onDiscard: () => void;
}) {
  return (
    <div className="mt-2.5 flex items-center gap-2">
      <button
        type="button"
        onClick={onConfirm}
        disabled={dispatching}
        className={cn(
          'inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] font-medium transition',
          dispatching
            ? 'cursor-not-allowed bg-muted text-muted-foreground'
            : 'bg-orange-500 text-white hover:bg-orange-500/90',
        )}
      >
        {dispatching ? (
          <Loading01Icon className="h-3 w-3 animate-spin" />
        ) : (
          <Tick01Icon className="h-3 w-3" />
        )}
        {dispatching ? 'Starting…' : 'Approve & run'}
        {!dispatching && runCount > 0 ? (
          <span className="ml-1 opacity-70">
            · {runCount} run{runCount === 1 ? '' : 's'}
          </span>
        ) : null}
      </button>
      <button
        type="button"
        onClick={onEdit}
        disabled={dispatching}
        className="inline-flex items-center gap-1 rounded-md border border-border/70 bg-background/80 px-2 py-1 text-[11px] font-medium text-foreground transition hover:border-foreground/30 hover:bg-muted/60 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <PencilEdit01Icon className="h-3 w-3" />
        Edit
      </button>
      <button
        type="button"
        onClick={onDiscard}
        disabled={dispatching}
        className="rounded-md px-2 py-1 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground disabled:opacity-50"
      >
        Discard
      </button>
    </div>
  );
}
