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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
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
import { Delete01Icon, PencilEdit01Icon, PlusSignIcon } from "@/lib/icons";
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
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editor, setEditor] = useState<Editor | null>(null);
  const [conflict, setConflict] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [editorRevision, setEditorRevision] = useState(0);
  const [lastSaved, setLastSaved] = useState(false);
  const pending =
    reloading ||
    createPipeline.isPending ||
    updatePipeline.isPending ||
    deletePipeline.isPending;
  const pipeline =
    pipelines.find((item) => item.id === selectedId) ??
    pipelines.find((item) => item.is_default) ??
    pipelines[0];

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
  ) => {
    if (pending || !editable) return;
    setLastSaved(false);
    try {
      await updatePipeline.mutateAsync({
        id: target.id,
        expected_updated_at: target.updated_at,
        ...data,
      });
      setEditor(null);
      setLastSaved(true);
      toast.success(success);
    } catch (error) {
      reportError(error);
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

  const deleteBlock = pipeline?.is_default
    ? "Choose another default pipeline before deleting this one."
    : (pipeline?.deal_count ?? 0) > 0
      ? "Move all deals to another pipeline before deleting this one, including won and lost deals."
      : null;

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
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2 sm:max-w-sm">
          <label
            htmlFor="pipeline-selector"
            className="text-xs font-medium text-muted-foreground"
          >
            Pipeline{pipelines.length > 0 ? ` · ${pipelines.length}` : ""}
          </label>
          {pipeline && (
            <Select
              value={pipeline.id}
              disabled={pending}
              onValueChange={(id) => {
                setSelectedId(id);
                setLastSaved(false);
              }}
            >
              <SelectTrigger
                variant="underline"
                id="pipeline-selector"
                className="w-full px-0.5"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {pipelines.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name}
                    {item.is_default ? " · Default" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
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

      {!pipeline ? (
        <QuietEmptyState
          title="Build your first sales pipeline"
          description="Start with a few stages that match how your team sells. You can rename, reorder, and add stages as your process evolves."
        />
      ) : (
        <>
          <div className="flex flex-wrap items-start justify-between gap-3 border-t pt-5">
            <div className="min-w-0 space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="break-words text-lg font-semibold tracking-tight">
                  {pipeline.name}
                </h2>
                {pipeline.is_default && (
                  <QuietStatusText>Default</QuietStatusText>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
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
              </p>
            </div>
            {editable && (
              <div className="flex items-center gap-1">
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
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h3 className="text-sm font-medium">Stages</h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {editable
                  ? "Drag a handle or choose a position to reorder stages within their outcome."
                  : "Stages define how deals progress through this pipeline."}
              </p>
            </div>
            <span className="text-xs text-muted-foreground" role="status">
              {pending
                ? "Saving changes…"
                : lastSaved
                  ? "All changes saved"
                  : editable
                    ? "Changes save automatically"
                    : "View only"}
            </span>
          </div>
          <PipelineStageTable
            key={pipeline.id}
            stages={pipeline.stages ?? []}
            editable={editable}
            pending={pending}
            onReorder={(stages) =>
              save(
                pipeline,
                { stages: stagePayload(stages) },
                "Stage order updated",
              )
            }
            onAdd={(type) => openEditor({ kind: "stage", pipeline, type })}
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
        </>
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
              setSelectedId(created.id);
              setEditor(null);
              setLastSaved(true);
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
          onSave={(data) =>
            save(
              editor.pipeline,
              data,
              editor.stage ? "Stage updated" : "Stage added",
            )
          }
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
          onSave={(data) => save(editor.pipeline, data, "Stage deleted")}
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
                    setSelectedId(null);
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
