import { useCallback, useMemo, useRef, useState } from "react";
import {
  ChatView,
  type ChatTaskSurface,
} from "@/components/agents/dock/ChatView";
import { useAgentRunStream } from "@/components/agents/dock/useAgentRunStream";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { QuietTextAction } from "@/components/design-system/quiet";
import { UpgradeRequiredDialog } from "@edition";
import {
  getUpgradeRequiredReason,
  type UpgradeRequiredReason,
} from "@edition/errors";
import { dockChatService } from "@/lib/services/dockChatService";
import type { DockChatDetail } from "@/lib/dockTypes";
import type {
  Agent,
  AutomationRule,
  CodingSessionInteraction,
  FlowTemplateManifest,
  WorkflowWithStates,
} from "@/lib/pmTypes";
import { FlowBuilderPreview } from "./FlowBuilderPreview";
import { FlowBuilderConfirmation } from "./FlowBuilderConfirmation";
interface Props {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template?: FlowTemplateManifest | null;
  flow?: AutomationRule | null;
  agents: Agent[];
  workflows: WorkflowWithStates[];
  onSaved: (flowId: string) => void;
  initialBrief?: string;
}
export function FlowBuilderDrawer(props: Props) {
  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side="right"
        className="w-full gap-0 p-0 data-[side=right]:w-full data-[side=right]:sm:w-[880px] data-[side=right]:!max-w-[880px]"
      >
        <SheetHeader className="border-b border-quiet-divider-strong px-6 py-5 pr-14">
          <SheetTitle>
            {props.flow
              ? "Edit flow"
              : (props.template?.name ?? "New custom flow")}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Describe your flow and review it before confirming.
          </SheetDescription>
        </SheetHeader>
        <FlowBuilderConversation {...props} />
      </SheetContent>
    </Sheet>
  );
}
function FlowBuilderConversation({
  workspaceId,
  open,
  onOpenChange,
  template,
  flow,
  agents,
  workflows,
  onSaved,
  initialBrief,
}: Props) {
  const [chatId, setChatId] = useState<string>();
  const [runId, setRunId] = useState<string | null>(null);
  const [upgrade, setUpgrade] = useState<UpgradeRequiredReason | null>(null);
  const [savedFlowId, setSavedFlowId] = useState<string>();
  const notified = useRef<string | undefined>(undefined);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const creation = useRef<Promise<{
    id: string;
  } | null> | null>(null);
  const showError = useCallback((error: unknown) => {
    const reason = getUpgradeRequiredReason(error);
    if (!reason) return false;
    setUpgrade(reason);
    return true;
  }, []);
  const onCreateChat = useCallback(() => {
    if (!creation.current)
      creation.current = dockChatService
        .createFlowBuilder(workspaceId, {
          templateKey: template?.key,
          flowId: flow?.id,
        })
        .then((result) => {
          if (result.error || !result.data) {
            creation.current = null;
            throw new Error(result.error ?? "Could not open the flow builder");
          }
          setChatId(result.data.id);
          return { id: result.data.id };
        })
        .catch((error) => {
          creation.current = null;
          throw error;
        });
    return creation.current;
  }, [workspaceId, template?.key, flow?.id]);
  const fetchers = useMemo(
    () => ({
      requestKey: `flow-builder:${chatId}`,
      getSnapshot: (ws: string, _run: string, signal?: AbortSignal) =>
        dockChatService.getChatRun(ws, chatId!, signal),
      listEvents: (
        ws: string,
        _run: string,
        after: number,
        signal?: AbortSignal,
      ) => dockChatService.listChatRunEvents(ws, chatId!, after, signal),
    }),
    [chatId],
  );
  const stream = useAgentRunStream(
    workspaceId,
    runId,
    open && !!chatId,
    3000,
    fetchers,
  );
  const detailChanged = useCallback(
    (detail: DockChatDetail | null) => {
      const id = detail?.chat.flow_builder?.created_flow_id;
      if (id && notified.current !== id) {
        notified.current = id;
        setSavedFlowId(id);
        onSaved(id);
      }
    },
    [onSaved],
  );
  const preview = useCallback(
    (detail: DockChatDetail | null) => {
      const state = detail?.chat.flow_builder;
      return state?.draft ? (
        <FlowBuilderPreview
          state={state}
          agents={agents}
          workflows={workflows}
        />
      ) : null;
    },
    [agents, workflows],
  );
  const taskSurface = useMemo<ChatTaskSurface>(
    () => ({
      emptyStateLayout: flow || template ? undefined : 'form',
      initialMessage: flow
        ? "Describe the current flow and ask what I would like to change."
        : template
          ? `Help me set up the ${template.name} template. Ask for the details you need.`
          : undefined,
      emptyState:
        flow || template ? (
          <span>
            {flow ? "Reading this flow…" : "Preparing your template…"}
          </span>
        ) : (
          <span className="block text-left">
            <strong className="block text-base font-semibold text-foreground">
              What would you like to automate?
            </strong>
            <span className="mt-1 block">
              Describe when it should run and what it should do.
            </span>
          </span>
        ),
      placeholder: flow
        ? "What would you like to change?"
        : chatId
          ? "Reply or ask for a change…"
          : "Describe your flow…",
      onDetailChange: detailChanged,
      onError: showError,
      // Drafts can be validation probes; show the summary only for final approval.
      renderInteraction: (interaction, detail, resolve) => {
        if (!isFlowApproval(interaction)) return undefined;
        const payload = interaction.request_payload as {
          action?: {
            revision?: string;
          };
          raw_input?: {
            action?: {
              revision?: string;
            };
          };
        };
        const revision =
          payload.action?.revision ?? payload.raw_input?.action?.revision;
        const valid =
          !!revision && revision === detail?.chat.flow_builder?.revision;
        return (
          <FlowBuilderConfirmation
            key={interaction.interaction_id}
            editing={!!flow}
            valid={valid}
            preview={valid ? preview(detail) : null}
            onResolve={(decision, message) =>
              resolve(interaction.interaction_id, {
                response_payload: { decision },
                followup_message: message,
              })
            }
          />
        );
      },
    }),
    [flow, template, chatId, detailChanged, showError, preview],
  );
  return (
    <>
      <div className="mx-auto flex min-h-0 w-full max-w-[744px] flex-1 flex-col px-1 sm:px-4">
        <ChatView
          workspaceId={workspaceId}
          chatId={chatId}
          active={open}
          onCreateChat={onCreateChat}
          scrollToLatestRequest={0}
          textareaRef={textareaRef}
          initialDraft={initialBrief}
          streamController={stream}
          onRunIdChange={setRunId}
          showComposerShortcutHint={false}
          taskSurface={taskSurface}
          readOnly={!!savedFlowId}
          starterSuggestions={
            flow || template
              ? []
              : [
                  {
                    label: "Review pull requests",
                    prompt:
                      "When a pull request opens, review it and post feedback.",
                  },
                  {
                    label: "Run on a schedule",
                    prompt:
                      "Every weekday morning, summarize our team’s recent work.",
                  },
                ]
          }
        />
      </div>
      {savedFlowId && (
        <div className="flex justify-end border-t border-quiet-divider-strong px-6 py-4">
          <QuietTextAction onClick={() => onOpenChange(false)}>
            Done
          </QuietTextAction>
        </div>
      )}
      {upgrade && (
        <UpgradeRequiredDialog
          open
          onOpenChange={(value) => {
            if (!value) setUpgrade(null);
          }}
          reason={upgrade}
        />
      )}
    </>
  );
}
function isFlowApproval(interaction: CodingSessionInteraction | null) {
  const payload = interaction?.request_payload as
    | {
        phase?: string;
        kind?: string;
      }
    | undefined;
  return (
    interaction?.interaction_kind === "approval_request" &&
    (payload?.phase === "flow_confirm" || payload?.kind === "flow_confirm")
  );
}
