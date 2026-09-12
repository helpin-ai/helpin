import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  QuietUnderlineInput as Input,
  QuietTextAction,
  QuietPrimaryAction,
} from "@/components/design-system/quiet";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import type {
  CRMDealCommercialMotion,
  CRMPipeline,
  CRMPipelineStage,
  CreateCRMPipelineRequest,
  PipelineStageType,
  UpdateCRMPipelineRequest,
} from "@/lib/crmTypes";
import {
  DEFAULT_STAGES,
  deletionBlock,
  DEAL_TYPE_OPTIONS,
  pipelineDealType,
  MOTION_DESCRIPTIONS,
  sortStages,
  stagePayload,
  STAGE_LABELS,
  STAGE_TYPES,
} from "./pipelineSettingsUtils";

type DialogProps = {
  pending: boolean;
  onClose: () => void;
  onReload?: () => void;
};

function ConflictNotice({
  onReload,
  pending,
}: {
  onReload?: () => void;
  pending: boolean;
}) {
  if (!onReload) return null;
  return (
    <div
      role="alert"
      className="space-y-2 border-y border-quiet-divider-strong py-3 text-sm text-quiet-text-secondary"
    >
      <p>
        This pipeline changed while you were editing. Reload to review the
        latest version. Your unsaved changes will be discarded.
      </p>
      <QuietTextAction disabled={pending} onClick={onReload}>
        Reload editor
      </QuietTextAction>
    </div>
  );
}
type SavePipeline = (data: UpdateCRMPipelineRequest) => Promise<void>;

export function PipelineDetailsDialog({
  pipeline,
  pipelines,
  pending,
  onClose,
  onSave,
  onReload,
}: DialogProps & {
  pipeline?: CRMPipeline;
  pipelines: CRMPipeline[];
  onSave: (
    data: Omit<CreateCRMPipelineRequest, "workspace_id">,
  ) => Promise<void>;
}) {
  const [name, setName] = useState(pipeline?.name ?? "");
  const [isDefault, setIsDefault] = useState(
    pipeline?.is_default ?? pipelines.length === 0,
  );
  const [motion, setMotion] = useState<CRMDealCommercialMotion>(
    pipeline?.default_commercial_motion ?? "new_business",
  );
  const dealType = pipelineDealType(motion);
  const [template, setTemplate] = useState("standard");
  const sourceStages =
    template === "standard"
      ? DEFAULT_STAGES
      : sortStages(
          pipelines.find((item) => item.id === template)?.stages ?? [],
        );

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {pipeline ? "Pipeline details" : "Create pipeline"}
          </DialogTitle>
          <DialogDescription>
            {pipeline
              ? "Set the name and defaults for this pipeline."
              : "Start with a sales process, then tailor its stages to your team."}
          </DialogDescription>
        </DialogHeader>
        <ConflictNotice onReload={onReload} pending={pending} />
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (!pending && name.trim())
              void onSave({
                name: name.trim(),
                is_default: isDefault,
                default_commercial_motion: motion,
                ...(!pipeline
                  ? {
                      stages: sourceStages.map(
                        ({ name, stage_type, probability }, position) => ({
                          name,
                          stage_type,
                          probability,
                          position,
                        }),
                      ),
                    }
                  : {}),
              });
          }}
          className="space-y-5"
        >
          <fieldset disabled={pending} className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="pipeline-name">Pipeline name</Label>
              <Input
                autoFocus
                id="pipeline-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
                maxLength={200}
                placeholder="e.g. Outbound sales"
              />
            </div>
            {!pipeline && (
              <div className="space-y-2">
                <Label htmlFor="pipeline-template">Start with</Label>
                <Select
                  disabled={pending}
                  value={template}
                  onValueChange={setTemplate}
                >
                  <SelectTrigger
                    variant="underline"
                    className="w-full px-0.5"
                    id="pipeline-template"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="standard">
                      Standard sales stages
                    </SelectItem>
                    {pipelines.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        Copy stages from {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <div className="border-l border-quiet-divider-strong pl-3 text-xs leading-relaxed text-muted-foreground">
                  {sourceStages.map((stage) => stage.name).join(" → ")}
                </div>
                <p className="text-xs text-muted-foreground">
                  Only stage names, order, outcomes, and probabilities are
                  copied. Deals stay in their original pipeline.
                </p>
              </div>
            )}
            <details className="group border-t border-quiet-divider-strong pt-3">
              <summary className="cursor-pointer text-sm text-quiet-text-secondary focus-visible:outline-2 focus-visible:outline-ring">
                Optional settings
              </summary>
              <div className="mt-4 space-y-5">
                <div className="space-y-2">
                  <Label htmlFor="pipeline-motion">Default deal type</Label>
                  <Select
                    disabled={pending}
                    value={dealType}
                    onValueChange={(value) =>
                      setMotion(value as CRMDealCommercialMotion)
                    }
                  >
                    <SelectTrigger
                      variant="underline"
                      className="w-full px-0.5"
                      id="pipeline-motion"
                      aria-describedby="pipeline-motion-help"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {DEAL_TYPE_OPTIONS.map(({ value, label }) => (
                        <SelectItem key={value} value={value}>
                          {label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p
                    id="pipeline-motion-help"
                    className="text-xs text-muted-foreground"
                  >
                    {MOTION_DESCRIPTIONS[dealType]} Deals inherit this type
                    unless they have their own. Changing the default also
                    affects existing deals that inherit it.
                  </p>
                </div>
                {(motion === "renewal" || motion === "expansion") && (
                  <p className="text-xs text-muted-foreground">
                    This pipeline’s saved{" "}
                    {motion === "renewal" ? "Renewal" : "Expansion"} type is
                    preserved unless you change this setting.
                  </p>
                )}
                <div className="flex items-start justify-between gap-4 border-t border-quiet-divider-strong py-3">
                  <div>
                    <Label htmlFor="pipeline-default">Default pipeline</Label>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {pipeline?.is_default
                        ? "To change the default, choose another pipeline and make it the default."
                        : "Preselected when your team creates a deal."}
                    </p>
                  </div>
                  <Switch
                    id="pipeline-default"
                    checked={isDefault}
                    onCheckedChange={setIsDefault}
                    disabled={!!pipeline?.is_default || pipelines.length === 0}
                  />
                </div>
              </div>
            </details>
          </fieldset>
          <DialogFooter>
            <QuietTextAction type="button" disabled={pending} onClick={onClose}>
              Cancel
            </QuietTextAction>
            <QuietPrimaryAction
              type="submit"
              disabled={pending || !name.trim()}
            >
              {pending
                ? "Saving…"
                : pipeline
                  ? "Save pipeline"
                  : "Create pipeline"}
            </QuietPrimaryAction>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function PipelineStageDialog({
  pipeline,
  stage,
  initialType,
  pending,
  onClose,
  onSave,
  onReload,
}: DialogProps & {
  pipeline: CRMPipeline;
  stage?: CRMPipelineStage;
  initialType: PipelineStageType;
  onSave: SavePipeline;
}) {
  const [name, setName] = useState(stage?.name ?? "");
  const [type, setType] = useState(stage?.stage_type ?? initialType);
  const [probability, setProbability] = useState(
    String(stage?.probability ?? (initialType === "won" ? 100 : 0)),
  );
  const stages = sortStages(pipeline.stages ?? []);
  const [position, setPosition] = useState(
    stage
      ? stages
          .filter((s) => s.stage_type === stage.stage_type)
          .findIndex((s) => s.id === stage.id)
      : stages.filter((s) => s.stage_type === initialType).length,
  );
  const otherStages = stages.filter((candidate) => candidate.id !== stage?.id);
  const group = otherStages.filter(
    (candidate) => candidate.stage_type === type,
  );
  const effectiveProbability =
    type === "won" ? 100 : type === "lost" ? 0 : Number(probability);
  const validProbability =
    (type !== "open" || probability.trim() !== "") &&
    Number.isInteger(effectiveProbability) &&
    effectiveProbability >= 0 &&
    effectiveProbability <= 100;
  const typeBlock =
    stage &&
    ((stage.deal_count ?? 0) > 0
      ? "Move deals out of this stage before changing its outcome."
      : deletionBlock(stages, stage));

  const save = () => {
    if (pending || !name.trim() || !validProbability) return;
    const updated = {
      ...stage,
      id: stage?.id ?? "",
      pipeline_id: pipeline.id,
      name: name.trim(),
      stage_type: type,
      probability: effectiveProbability,
      position: 0,
      created_at: stage?.created_at ?? "",
      updated_at: stage?.updated_at ?? "",
    };
    const nextGroup = [...group];
    nextGroup.splice(position, 0, updated);
    const next = STAGE_TYPES.flatMap((candidate) =>
      candidate === type
        ? nextGroup
        : otherStages.filter((s) => s.stage_type === candidate),
    );
    void onSave({
      stages: stagePayload(next),
      expected_updated_at: pipeline.updated_at,
    });
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{stage ? "Edit stage" : "Add stage"}</DialogTitle>
          <DialogDescription>
            {stage
              ? `Update this stage in ${pipeline.name}.`
              : `Add a step to ${pipeline.name}.`}
          </DialogDescription>
        </DialogHeader>
        <ConflictNotice onReload={onReload} pending={pending} />
        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            save();
          }}
        >
          <fieldset disabled={pending} className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="stage-name">Stage name</Label>
              <Input
                autoFocus
                id="stage-name"
                required
                maxLength={200}
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="e.g. Discovery call"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="stage-type">Outcome</Label>
                <Select
                  value={type}
                  disabled={pending || !!typeBlock}
                  onValueChange={(value) => {
                    const nextType = value as PipelineStageType;
                    setType(nextType);
                    setPosition(
                      otherStages.filter((s) => s.stage_type === nextType)
                        .length,
                    );
                  }}
                >
                  <SelectTrigger
                    variant="underline"
                    className="w-full px-0.5"
                    id="stage-type"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {STAGE_TYPES.map((value) => (
                      <SelectItem value={value} key={value}>
                        {STAGE_LABELS[value]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="stage-probability">Win probability (%)</Label>
                <Input
                  id="stage-probability"
                  type="number"
                  min={0}
                  max={100}
                  step={1}
                  required
                  disabled={type !== "open"}
                  value={type === "open" ? probability : effectiveProbability}
                  onChange={(event) => setProbability(event.target.value)}
                />
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              {typeBlock ||
                (type === "open"
                  ? "Open stages track deals in progress. Set their estimated chance of winning."
                  : `${STAGE_LABELS[type]} stages close the deal and use ${effectiveProbability}% probability.`)}
            </p>
            <div className="space-y-2">
              <Label htmlFor="stage-position">
                Position in {STAGE_LABELS[type].toLowerCase()} stages
              </Label>
              <Select
                disabled={pending}
                value={String(position)}
                onValueChange={(value) => setPosition(Number(value))}
              >
                <SelectTrigger
                  variant="underline"
                  className="w-full px-0.5"
                  id="stage-position"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">1 · First stage</SelectItem>
                  {group.map((candidate, index) => (
                    <SelectItem value={String(index + 1)} key={candidate.id}>
                      {index + 2} · After {candidate.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </fieldset>
          <DialogFooter>
            <QuietTextAction type="button" disabled={pending} onClick={onClose}>
              Cancel
            </QuietTextAction>
            <QuietPrimaryAction
              type="submit"
              disabled={pending || !name.trim() || !validProbability}
            >
              {pending ? "Saving…" : stage ? "Save stage" : "Add stage"}
            </QuietPrimaryAction>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function DeletePipelineStageDialog({
  pipeline,
  stage,
  pending,
  onClose,
  onSave,
  onReload,
}: DialogProps & {
  pipeline: CRMPipeline;
  stage: CRMPipelineStage;
  onSave: SavePipeline;
}) {
  const [destination, setDestination] = useState("");
  const count = stage.deal_count ?? 0;
  const remaining = sortStages(pipeline.stages ?? []).filter(
    (candidate) => candidate.id !== stage.id,
  );
  const destinations = remaining.filter(
    (candidate) => candidate.stage_type === stage.stage_type,
  );
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Delete “{stage.name}”?</DialogTitle>
          <DialogDescription>
            {count > 0
              ? `${count} ${count === 1 ? "deal uses" : "deals use"} this stage. Choose where to move ${count === 1 ? "it" : "them"} before removing it.`
              : "This stage has no deals. Removing it cannot be undone."}
          </DialogDescription>
        </DialogHeader>
        <ConflictNotice onReload={onReload} pending={pending} />
        <div className="space-y-3">
          {destinations.length > 0 && (
            <div className="space-y-2">
              <Label htmlFor="stage-destination">
                Move deals to{count === 0 ? " (optional)" : ""}
              </Label>
              <Select
                value={destination}
                onValueChange={setDestination}
                disabled={pending}
              >
                <SelectTrigger
                  variant="underline"
                  className="w-full px-0.5"
                  id="stage-destination"
                >
                  <SelectValue placeholder="Choose a stage" />
                </SelectTrigger>
                <SelectContent>
                  {destinations.map((candidate) => (
                    <SelectItem key={candidate.id} value={candidate.id}>
                      {candidate.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <p className="border-l border-quiet-divider-strong pl-3 text-sm leading-relaxed text-muted-foreground">
            {count > 0
              ? "Your deals will be kept. Only stages with the same outcome are available, so open deals stay open and closed deals keep their outcome."
              : "If deals arrive before deletion, they will move to your chosen stage. Without a destination, deletion will be stopped."}
          </p>
        </div>
        <DialogFooter>
          <QuietTextAction disabled={pending} onClick={onClose}>
            Cancel
          </QuietTextAction>
          <Button
            variant="destructive"
            disabled={pending || (count > 0 && !destination)}
            onClick={() =>
              void onSave({
                stages: stagePayload(remaining),
                expected_updated_at: pipeline.updated_at,
                stage_migrations: destination
                  ? { [stage.id]: destination }
                  : undefined,
              })
            }
          >
            {pending
              ? "Deleting…"
              : count > 0 || destination
                ? "Move deals & delete stage"
                : "Delete stage"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
