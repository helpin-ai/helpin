import type { CommandBarPlanStep, CommandBarPlanSummary } from '@/lib/pmTypes';

/**
 * Live, dock-friendly shape of a command-bar plan. This is what the rail
 * visualizers (`DeliveryPlanView`, `FanOutRail`, etc.) consume — a camelCased
 * projection of the API's snake_cased `CommandBarPlanSummary`.
 */
export interface CommandBarRunPlan {
  id: string;
  steps: CommandBarPlanStep[];
  runIdsByStep: Record<number, string>;
  planKind?: CommandBarPlanSummary['plan_kind'];
  status?: CommandBarPlanSummary['status'];
  prompt?: string;
  errorMessage?: string;
  currentStepIndex?: number;
  createdAt?: string;
  updatedAt?: string;
  aiProfileId?: string;
  aiProfileOwnerId?: string;
}

/** Maps an API plan summary into the rail-friendly `CommandBarRunPlan` shape. */
export function planSummaryToRunPlan(plan: CommandBarPlanSummary): CommandBarRunPlan {
  return {
    id: plan.id,
    steps: plan.steps,
    runIdsByStep: plan.run_ids_by_step ?? {},
    planKind: plan.plan_kind,
    status: plan.status,
    prompt: plan.prompt,
    errorMessage: plan.error_message,
    currentStepIndex: plan.current_step_index,
    createdAt: plan.created_at,
    updatedAt: plan.updated_at,
    aiProfileId: plan.ai_profile_id,
    aiProfileOwnerId: plan.ai_profile_owner_id,
  };
}
