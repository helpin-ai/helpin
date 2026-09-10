import type {
  CRMDealCommercialMotion,
  CRMPipelineStage,
  PipelineStageType,
} from "@/lib/crmTypes";

export const STAGE_TYPES: PipelineStageType[] = ["open", "won", "lost"];
export const STAGE_LABELS: Record<PipelineStageType, string> = {
  open: "Open",
  won: "Won",
  lost: "Lost",
};
export const MOTION_LABELS: Record<CRMDealCommercialMotion, string> = {
  new_business: "New business",
  expansion: "Expansion",
  renewal: "Renewal",
};
export const DEFAULT_STAGES = [
  {
    name: "Qualification",
    stage_type: "open" as const,
    probability: 10,
    position: 0,
  },
  {
    name: "Proposal",
    stage_type: "open" as const,
    probability: 30,
    position: 1,
  },
  {
    name: "Negotiation",
    stage_type: "open" as const,
    probability: 60,
    position: 2,
  },
  {
    name: "Closed won",
    stage_type: "won" as const,
    probability: 100,
    position: 3,
  },
  {
    name: "Closed lost",
    stage_type: "lost" as const,
    probability: 0,
    position: 4,
  },
];

export function sortStages(stages: CRMPipelineStage[]) {
  return [...stages].sort(
    (a, b) =>
      STAGE_TYPES.indexOf(a.stage_type) - STAGE_TYPES.indexOf(b.stage_type) ||
      a.position - b.position,
  );
}

export function stagePayload(stages: CRMPipelineStage[]) {
  return stages.map((stage, position) => ({
    id: stage.id || undefined,
    name: stage.name,
    stage_type: stage.stage_type,
    probability: stage.probability,
    position,
  }));
}

export function deletionBlock(
  stages: CRMPipelineStage[],
  stage: CRMPipelineStage,
) {
  return stages.filter((candidate) => candidate.stage_type === stage.stage_type)
    .length <= 1
    ? `Keep at least one ${STAGE_LABELS[stage.stage_type].toLowerCase()} stage. Add another before removing this one.`
    : null;
}
