import type { Agent, AgentRun, Epic } from '@/lib/pmTypes';

export type PlanningStep = 'setup' | 'draft' | 'approve' | 'generate' | 'review' | 'execute';
export type StepStatus = 'completed' | 'current' | 'upcoming';

export const STEP_LABELS: Record<PlanningStep, string> = {
  setup: 'Set up planner',
  draft: 'Draft spec',
  approve: 'Approve spec',
  generate: 'Generate stories',
  review: 'Review & confirm',
  execute: 'Start execution',
};

export const STEPS: PlanningStep[] = ['setup', 'draft', 'approve', 'generate', 'review', 'execute'];

function getRunStage(run: AgentRun): string {
  const stage = run.input?.stage;
  return typeof stage === 'string' ? stage : '';
}

function hasActiveStageRun(runs: AgentRun[], stage: string): boolean {
  return runs.some((run) => getRunStage(run) === stage && ['queued', 'running'].includes(run.status));
}

export function computeCurrentStep(epic: Epic, _agents: Agent[], runs: AgentRun[]): PlanningStep {
  if (!epic.orchestrator_agent_id) return 'setup';

  if (hasActiveStageRun(runs, 'draft_spec')) return 'draft';
  if (hasActiveStageRun(runs, 'plan_stories')) return 'generate';

  if (epic.planning_state === 'awaiting_spec_approval') return 'approve';
  if (!epic.approved_spec_version_id) return 'draft';
  if (epic.planning_state === 'ready_for_story_planning') return 'generate';
  if (epic.planning_state === 'awaiting_plan_approval') return 'review';
  if (epic.planning_state === 'stories_created' || epic.planning_state === 'execution_started' || epic.planning_state === 'ready_for_execution') return 'execute';
  // Fallback: if spec is approved but state hasn't caught up
  if (epic.approved_spec_version_id && !epic.planning_state?.includes('story') && !epic.planning_state?.includes('execution')) return 'generate';
  return 'setup';
}

export function getStepStatus(step: PlanningStep, currentStep: PlanningStep): StepStatus {
  const currentIndex = STEPS.indexOf(currentStep);
  const stepIndex = STEPS.indexOf(step);
  if (stepIndex < currentIndex) return 'completed';
  if (stepIndex === currentIndex) return 'current';
  return 'upcoming';
}

export function humanizeError(raw: string | undefined): string {
  if (!raw) return 'An unexpected error occurred.';
  if (raw.includes('failed to parse')) return 'The AI produced an invalid response. Please try again.';
  if (raw.includes('context deadline') || raw.includes('timeout')) return 'The operation timed out. Please try again.';
  if (raw.includes('rate limit')) return 'Rate limit reached. Please wait a moment and try again.';
  return 'Something went wrong. Check the activity log for details.';
}
