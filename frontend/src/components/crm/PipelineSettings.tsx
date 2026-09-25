import { useState } from "react";
import { toast } from "sonner";
import {
  QuietEmptyState,
  QuietTextAction,
  QuietStatusText,
} from "@/components/design-system/quiet";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  useCreatePipeline,
  useDeletePipeline,
  usePermissions,
  usePipelines,
  useUpdatePipeline,
  useWorkspaceAccess,
} from "@/hooks/queries";
import {
  ArrowDown01Icon,
  Delete01Icon,
  PencilEdit01Icon,
  PlusSignIcon,
} from "@/lib/icons";
import type {
  CRMPipeline,
  CRMPipelineStage,
  PipelineStageType,
  UpdateCRMPipelineRequest,
} from "@/lib/crmTypes";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { PipelineStageTable } from "./pipelines/PipelineStageTable";
import {
  DeletePipelineStageDialog,
  PipelineDetailsDialog,
  PipelineStageDialog,
} from "./pipelines/PipelineSettingsDialogs";
import { MOTION_LABELS, stagePayload } from "./pipelines/pipelineSettingsUtils";

type Editor =
  | { kind: "pipeline"; pipeline?: CRMPipeline }
  | {
      kind: "stage";
      pipeline: CRMPipeline;
      stage?: CRMPipelineStage;
      type: PipelineStageType;
    }
  | { kind: "delete-stage"; pipeline: CRMPipeline; stage: CRMPipelineStage }
  | { kind: "delete-pipeline"; pipeline: CRMPipeline };

export function PipelineSettings() {
  const wsId = useWorkspaceStore((state) => state.currentWorkspace?.id ?? "");
  const {
    data: pipelines = [],
    isLoading,
    isError,
    isLoadingError,
    refetch,
  } = usePipelines(wsId);
  const { data: access } = useWorkspaceAccess(wsId);
  const editable = usePermissions(access).has("crm.admin");
  const createPipeline = useCreatePipeline(wsId);
  const updatePipeline = useUpdatePipeline(wsId);
  const deletePipeline = useDeletePipeline(wsId);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [editor, setEditor] = useState<Editor | null>(null);
  const [conflict, setConflict] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [editorRevision, setEditorRevision] = useState(0);
  const [stageEditVersions, setStageEditVersions] = useState<
    Record<string, number>
  >({});
  const pending =
    reloading ||
    createPipeline.isPending ||
    updatePipeline.isPending ||
    deletePipeline.isPending;

  const openEditor = (next: Editor) => {
    setConflict(false);
    setEditor(next);
  };
  const reloadEditor = async () => {
    if (!editor || !editor.pipeline) return;
    setReloading(true);
    try {
      const result = await refetch();
      if (result.error) throw result.error;
      const latest = result.data?.find(
        (item) => item.id === editor.pipeline?.id,
      );
      if (!latest) {
        setEditor(null);
        return;
      }
      if (editor.kind === "stage" || editor.kind === "delete-stage") {
        const stage = latest.stages?.find(
          (item) => item.id === editor.stage?.id,
        );
        if (editor.stage && !stage) {
          setEditor(null);
          toast.info("This stage has already been removed.");
          return;
        }
        if (editor.kind === "delete-stage" && stage)
          setEditor({ ...editor, pipeline: latest, stage });
        else if (editor.kind === "stage")
          setEditor({ ...editor, pipeline: latest, stage });
      } else {
        setEditor({ ...editor, pipeline: latest });
      }
      setConflict(false);
      setEditorRevision((value) => value + 1);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Could not reload the pipeline.",
      );
    } finally {
      setReloading(false);
    }
  };
  const reportError = (error: unknown) => {
    if (error instanceof Error && "status" in error && error.status === 409)
      setConflict(true);
    toast.error(
      error instanceof Error
        ? error.message
        : "Could not save changes. Please try again.",
    );
  };
  const save = async (
    target: CRMPipeline,
    data: UpdateCRMPipelineRequest,
    success: string,
  ): Promise<boolean> => {
    if (pending || !editable) return false;
    try {
      await updatePipeline.mutateAsync({
        id: target.id,
        expected_updated_at: target.updated_at,
        ...data,
      });
      setEditor(null);
      if (editor?.kind === "stage" && editor.stage) {
        const stageId = editor.stage.id;
        setStageEditVersions((versions) => ({
          ...versions,
          [stageId]: (versions[stageId] ?? 0) + 1,
        }));
      }
      toast.success(success);
      return true;
    } catch (error) {
      reportError(error);
      return false;
    }
  };

  if (isLoading)
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  if (isLoadingError)
    return (
      <div className="rounded-xl border p-6 text-center">
        <p className="mb-3 text-sm text-muted-foreground">
          Could not load pipelines.
        </p>
        <Button variant="outline" onClick={() => void refetch()}>
          Try again
        </Button>
      </div>
    );

  return (
    <div className="space-y-5">
      {isError && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-2 border-b border-quiet-divider-strong pb-3 text-sm text-quiet-text-secondary"
        >
          <span>
            Could not refresh pipelines. Showing the last loaded version.
          </span>
          <QuietTextAction onClick={() => void refetch()}>
            Try again
          </QuietTextAction>
        </div>
      )}
      <div className="flex justify-end">
        {editable && (
          <QuietTextAction
            disabled={pending}
            onClick={() => openEditor({ kind: "pipeline" })}
          >
            <PlusSignIcon className="size-4" />
            Create pipeline
          </QuietTextAction>
        )}
      </div>

      {pipelines.length === 0 ? (
        <QuietEmptyState
          title="Build your first sales pipeline"
          description="Start with a few stages that match how your team sells. You can rename, reorder, and add stages as your process evolves."
        />
      ) : (
        pipelines.map((pipeline) => {
          const expanded = pipeline.id === expandedId;
          const deleteBlock = pipeline.is_default
            ? "Choose another default pipeline before deleting this one."
            : (pipeline.deal_count ?? 0) > 0
              ? "Move all deals to another pipeline before deleting this one, including won and lost deals."
              : null;
          return (
            <section
              key={pipeline.id}
              className="group/pipeline overflow-hidden rounded-lg border border-border/70 bg-card"
            >
              <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4">
                <h2
                  id={`pipeline-${pipeline.id}`}
                  className="min-w-0 basis-full sm:flex-1 sm:basis-0"
                >
                  <span className="min-w-0 space-y-2">
                    <span className="flex flex-wrap items-center gap-2">
                      <span className="break-words text-sm font-semibold text-quiet-text-primary">
                        {pipeline.name}
                      </span>
                      {pipeline.is_default && (
                        <QuietStatusText>Default</QuietStatusText>
                      )}
                    </span>
                    <span className="block text-xs font-normal text-muted-foreground">
                      {
                        MOTION_LABELS[
                          pipeline.default_commercial_motion ?? "new_business"
                        ]
                      }
                      <span className="mx-2">·</span>
                      {pipeline.stages?.length ?? 0} stages
                      <span className="mx-2">·</span>
                      {pipeline.deal_count ?? 0}{" "}
                      {pipeline.deal_count === 1 ? "deal" : "deals"}
                    </span>
                  </span>
                </h2>
                {editable && (
                  <div
                    className={`ml-auto flex items-center gap-1 transition-opacity ${expanded ? "" : "pointer-events-none opacity-0 group-hover/pipeline:pointer-events-auto group-hover/pipeline:opacity-100 group-focus-within/pipeline:pointer-events-auto group-focus-within/pipeline:opacity-100"}`}
                  >
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={pending}
                      onClick={() => openEditor({ kind: "pipeline", pipeline })}
                    >
                      <PencilEdit01Icon className="size-3.5" />
                      Edit pipeline
                    </Button>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span
                          tabIndex={deleteBlock ? 0 : undefined}
                          aria-label={deleteBlock ?? undefined}
                        >
                          <Button
                            size="icon"
                            variant="ghost"
                            className="size-8 text-muted-foreground hover:text-destructive"
                            aria-label="Delete pipeline"
                            disabled={pending || !!deleteBlock}
                            onClick={() =>
                              openEditor({ kind: "delete-pipeline", pipeline })
                            }
                          >
                            <Delete01Icon className="size-4" />
                          </Button>
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        {deleteBlock ?? "Delete pipeline"}
                      </TooltipContent>
                    </Tooltip>
                  </div>
                )}
                <QuietTextAction
                  id={`pipeline-toggle-${pipeline.id}`}
                  aria-label={`${expanded ? "Hide" : "Show"} stages for ${pipeline.name}`}
                  aria-expanded={expanded}
                  aria-controls={`pipeline-stages-${pipeline.id}`}
                  disabled={pending}
                  onClick={() => setExpandedId(expanded ? null : pipeline.id)}
                  className="ml-auto shrink-0 sm:ml-0"
                >
                  {expanded ? "Hide stages" : "Show stages"}
                  <ArrowDown01Icon
                    className={`size-4 transition-transform ${expanded ? "rotate-180" : ""}`}
                  />
                </QuietTextAction>
              </div>
              <div
                id={`pipeline-stages-${pipeline.id}`}
                role="region"
                aria-labelledby={`pipeline-${pipeline.id}`}
                hidden={!expanded}
                className="border-t border-quiet-divider-strong px-4 pb-4 pt-2"
              >
                {expanded && (
                  <PipelineStageTable
                    key={pipeline.id}
                    stages={pipeline.stages ?? []}
                    editable={editable}
                    pending={pending}
                    editVersions={stageEditVersions}
                    onReorder={async (stages) => {
                      await save(
                        pipeline,
                        { stages: stagePayload(stages) },
                        "Stage order updated",
                      );
                    }}
                    onUpdate={(stage, changes) =>
                      save(
                        pipeline,
                        {
                          stages: stagePayload(
                            (pipeline.stages ?? []).map((item) =>
                              item.id === stage.id
                                ? { ...item, ...changes }
                                : item,
                            ),
                          ),
                        },
                        "Stage updated",
                      )
                    }
                    onAdd={(type) =>
                      openEditor({ kind: "stage", pipeline, type })
                    }
                    onEdit={(stage) =>
                      openEditor({
                        kind: "stage",
                        pipeline,
                        stage,
                        type: stage.stage_type,
                      })
                    }
                    onDelete={(stage) =>
                      openEditor({ kind: "delete-stage", pipeline, stage })
                    }
                  />
                )}
              </div>
            </section>
          );
        })
      )}

      {editor?.kind === "pipeline" && (
        <PipelineDetailsDialog
          key={editorRevision}
          onReload={conflict ? () => void reloadEditor() : undefined}
          pipeline={editor.pipeline}
          pipelines={pipelines}
          pending={pending}
          onClose={() => setEditor(null)}
          onSave={async (data) => {
            if (!editable || pending) return;
            if (editor.pipeline) {
              await save(editor.pipeline, data, "Pipeline updated");
              return;
            }
            try {
              const created = await createPipeline.mutateAsync({
                ...data,
                workspace_id: wsId,
              });
              setExpandedId(created.id);
              setEditor(null);
              toast.success("Pipeline created");
            } catch (error) {
              reportError(error);
            }
          }}
        />
      )}
      {editor?.kind === "stage" && (
        <PipelineStageDialog
          key={editorRevision}
          onReload={conflict ? () => void reloadEditor() : undefined}
          pipeline={editor.pipeline}
          stage={editor.stage}
          initialType={editor.type}
          pending={pending}
          onClose={() => setEditor(null)}
          onSave={async (data) => {
            await save(
              editor.pipeline,
              data,
              editor.stage ? "Stage updated" : "Stage added",
            );
          }}
        />
      )}
      {editor?.kind === "delete-stage" && (
        <DeletePipelineStageDialog
          key={editorRevision}
          onReload={conflict ? () => void reloadEditor() : undefined}
          pipeline={editor.pipeline}
          stage={editor.stage}
          pending={pending}
          onClose={() => setEditor(null)}
          onSave={async (data) => {
            await save(editor.pipeline, data, "Stage deleted");
          }}
        />
      )}
      {editor?.kind === "delete-pipeline" && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open && !pending) setEditor(null);
          }}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delete “{editor.pipeline.name}”?</DialogTitle>
              <DialogDescription>
                This pipeline and its stages will be permanently removed. This
                action cannot be undone.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button
                variant="outline"
                disabled={pending}
                onClick={() => setEditor(null)}
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                disabled={pending}
                onClick={async () => {
                  if (!editable || pending) return;
                  try {
                    await deletePipeline.mutateAsync(editor.pipeline.id);
                    setEditor(null);
                    setExpandedId((current) =>
                      current === editor.pipeline.id ? null : current,
                    );
                    toast.success("Pipeline deleted");
                  } catch (error) {
                    reportError(error);
                  }
                }}
              >
                {pending ? "Deleting…" : "Delete pipeline"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
