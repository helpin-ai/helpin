import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import { useWorkspaceAccess, usePermissions } from "@/hooks/queries/useSession";
import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { format } from "date-fns";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  QuietSearchInput,
  QuietSelect,
  QuietEmptyState,
  QuietUnderlineInput,
} from "@/components/design-system/quiet";
import {
  useSequenceEnrollments,
  useOutreachRefresh,
} from "@/hooks/queries/useCRMOutreach";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import { useAuthStore } from "@/stores/authStore";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { EmailBodyRenderer } from "@/components/support/EmailBodyRenderer";
import { EmailContentEditor } from "./EmailContentEditor";
import type {
  SequenceEnrollment,
  SequenceDelivery,
} from "@/lib/crmOutreachTypes";
const labels: Record<string, string> = {
  active: "Scheduled",
  paused: "Paused",
  needs_review: "Needs review",
  waiting_task: "Waiting for task",
  uncertain: "Check delivery",
  failed: "Needs attention",
  completed: "Completed",
  replied: "Replied",
  unsubscribed: "Unsubscribed",
  bounced: "Undeliverable",
  deal_closed: "Deal closed",
  stopped: "Stopped",
  sending: "Sending",
};
export function SequenceActivity({
  workspaceId,
  sequenceId,
  contactId,
  dealId,
}: {
  workspaceId: string;
  sequenceId?: string;
  contactId?: string;
  dealId?: string;
}) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [selected, setSelected] = useState<string>();
  const query = useSequenceEnrollments(workspaceId, {
    sequence_id: sequenceId,
    contact_id: contactId,
    deal_id: dealId,
    search,
    status,
  });
  const rows = query.data ?? [];
  return (
    <div className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <QuietSearchInput
          containerClassName="min-w-0 flex-1"
          placeholder="Search recipients"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <QuietSelect
          label="Recipient status"
          value={status}
          onChange={setStatus}
          options={[
            { value: "all", label: "All statuses" },
            ...Object.entries(labels).map(([value, label]) => ({
              value,
              label,
            })),
          ]}
        />
      </div>
      {query.isLoading ? (
        <p className="py-6 text-sm text-muted-foreground">
          Loading recipients…
        </p>
      ) : query.isError ? (
        <Button variant="ghost" onClick={() => void query.refetch()}>
          Couldn’t load recipients. Try again
        </Button>
      ) : rows.length ? (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border/60 text-xs text-muted-foreground">
              <tr>
                <th className="py-2 font-medium">Recipient</th>
                {!sequenceId && <th className="px-3 font-medium">Sequence</th>}
                <th className="px-3 font-medium">Status</th>
                <th className="px-3 font-medium">Progress</th>
                <th className="font-medium">Next action</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.id}
                  className="border-b border-border/40 hover:bg-muted/20"
                >
                  <td className="py-3">
                    <button
                      className="max-w-64 text-left"
                      onClick={() => setSelected(row.id)}
                    >
                      <span className="block truncate font-medium">
                        {row.contact_name}
                      </span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {row.email}
                      </span>
                    </button>
                  </td>
                  {!sequenceId && (
                    <td className="px-3 text-xs">{row.sequence_name}</td>
                  )}
                  <td className="whitespace-nowrap px-3 text-xs">
                    <span
                      className={
                        ["failed", "uncertain"].includes(row.status)
                          ? "text-destructive"
                          : row.status === "replied"
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-muted-foreground"
                      }
                    >
                      {labels[row.status] ?? row.status}
                    </span>
                  </td>
                  <td className="px-3 text-xs text-muted-foreground">
                    {Math.min(
                      row.step_index,
                      row.step_count ?? row.steps?.length ?? 0,
                    )}{" "}
                    / {row.step_count ?? row.steps?.length ?? 0}
                  </td>
                  <td className="whitespace-nowrap text-xs text-muted-foreground">
                    {row.status === "active" ? (
                      format(new Date(row.next_at), "MMM d, h:mm a")
                    ) : row.status === "needs_review" ? (
                      <button
                        className="underline"
                        onClick={() => setSelected(row.id)}
                      >
                        Review email
                      </button>
                    ) : row.status === "uncertain" ? (
                      <button
                        className="underline"
                        onClick={() => setSelected(row.id)}
                      >
                        Inspect
                      </button>
                    ) : (
                      "—"
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <QuietEmptyState
          title={
            search || status !== "all"
              ? "No matching recipients"
              : "No recipients yet"
          }
          description={
            search || status !== "all"
              ? "Try another search or status."
              : "Add contacts to a published sequence to see their progress here."
          }
        />
      )}
      {query.hasNextPage && (
        <div className="flex justify-center">
          <Button
            variant="ghost"
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            {query.isFetchingNextPage ? "Loading…" : "Load more recipients"}
          </Button>
        </div>
      )}
      {selected && (
        <RecipientDetail
          workspaceId={workspaceId}
          id={selected}
          onClose={() => setSelected(undefined)}
        />
      )}
    </div>
  );
}
function RecipientDetail({
  workspaceId,
  id,
  onClose,
}: {
  workspaceId: string;
  id: string;
  onClose: () => void;
}) {
  const [dirty, setDirty] = useState(false);
  const [discard, setDiscard] = useState(false);
  const query = useQuery({
    queryKey: ["crm", workspaceId, "sequenceRecipient", id],
    queryFn: async () =>
      unwrap(await crmOutreachService.detail(workspaceId, id)),
    refetchInterval: 15000,
  });
  return (
    <>
      <Dialog
        open
        onOpenChange={(open) => {
          if (!open) {
            if (dirty) setDiscard(true);
            else onClose();
          }
        }}
      >
        <DialogContent
          className="max-h-[90vh] overflow-auto sm:max-w-2xl"
          aria-describedby={undefined}
        >
          {query.data ? (
            <RecipientContent
              key={`${id}:${query.data.enrollment.step_index}:${query.data.enrollment.status}`}
              workspaceId={workspaceId}
              row={query.data.enrollment}
              deliveries={query.data.deliveries}
              onDirtyChange={setDirty}
            />
          ) : (
            <DialogHeader>
              <DialogTitle>Recipient activity</DialogTitle>
              <DialogDescription>
                {query.isError ? "Could not load this recipient." : "Loading…"}
              </DialogDescription>
            </DialogHeader>
          )}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={discard}
        onOpenChange={setDiscard}
        title="Discard review changes?"
        description="Your changes have not been approved or saved."
        confirmLabel="Discard changes"
        cancelLabel="Keep reviewing"
        onConfirm={onClose}
      />
    </>
  );
}
function RecipientContent({
  workspaceId,
  row,
  deliveries,
  onDirtyChange,
}: {
  workspaceId: string;
  row: SequenceEnrollment;
  deliveries: SequenceDelivery[];
  onDirtyChange: (dirty: boolean) => void;
}) {
  const access = useWorkspaceAccess(workspaceId);
  const canEdit = usePermissions(access.data).has("crm.edit");
  const user = useAuthStore((s) => s.user?.id);
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug);
  const refresh = useOutreachRefresh(workspaceId);
  const [busy, setBusy] = useState(false);
  const step = row.steps[row.step_index];
  const [subject, setSubject] = useState(step?.subject ?? "");
  const [body, setBody] = useState(step?.body_html ?? "");
  const reviewDirty =
    row.status === "needs_review" &&
    (subject !== (step?.subject ?? "") || body !== (step?.body_html ?? ""));
  useEffect(() => onDirtyChange(reviewDirty), [reviewDirty, onDirtyChange]);
  const terminal = [
    "completed",
    "stopped",
    "replied",
    "unsubscribed",
    "bounced",
    "deal_closed",
  ].includes(row.status);
  const action = async (name: string) => {
    setBusy(true);
    try {
      unwrap(
        await crmOutreachService.control(
          workspaceId,
          row.id,
          name,
          name === "approve" ? { subject, body_html: body } : undefined,
        ),
      );
      await refresh();
      toast.success(
        name === "approve"
          ? "Email approved"
          : name === "check_delivery"
            ? "Delivery confirmed"
            : "Recipient updated",
      );
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not update recipient",
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <DialogHeader>
        <DialogTitle>{row.contact_name}</DialogTitle>
        <DialogDescription>
          {row.email} · {row.sequence_name}
        </DialogDescription>
      </DialogHeader>
      <div className="flex flex-wrap items-center gap-2">
        <span className="mr-auto text-sm font-medium">
          {labels[row.status] ?? row.status}
        </span>
        {row.owner_id === user && canEdit && !terminal && (
          <>
            {row.status === "uncertain" ? (
              <Button
                size="sm"
                disabled={busy}
                onClick={() => void action("check_delivery")}
              >
                Check delivery
              </Button>
            ) : ["paused", "failed"].includes(row.status) ? (
              <Button
                size="sm"
                disabled={busy}
                onClick={() => void action("resume")}
              >
                Resume
              </Button>
            ) : (
              <Button
                size="sm"
                variant="outline"
                disabled={busy}
                onClick={() => void action("pause")}
              >
                Pause
              </Button>
            )}
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => void action("stop")}
            >
              Stop
            </Button>
          </>
        )}
      </div>
      {row.error && <p className="text-sm text-destructive">{row.error}</p>}
      {row.status === "needs_review" && step && (
        <section className="space-y-3 border-y border-border/50 py-4">
          <h3 className="text-sm font-medium">Review next email</h3>
          {row.owner_id === user && canEdit ? (
            <>
              <QuietUnderlineInput
                aria-label="Review subject"
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                disabled={busy}
              />
              <EmailContentEditor
                workspaceId={workspaceId}
                value={body}
                onChange={setBody}
                disabled={busy}
              />
              <div className="flex justify-end">
                <Button disabled={busy} onClick={() => void action("approve")}>
                  Approve email
                </Button>
              </div>
            </>
          ) : (
            <EmailBodyRenderer
              html={step.body_html ?? ""}
              collapsedByDefault={false}
            />
          )}
        </section>
      )}
      <div className="space-y-4">
        {row.steps.map((step, i) => {
          const delivery = deliveries.find((d) => d.step_index === i);
          return (
            <div key={i} className="border-b border-border/40 pb-3">
              <div className="flex items-start gap-2">
                <span className="text-xs text-muted-foreground">{i + 1}</span>
                <span className="min-w-0 flex-1 text-sm font-medium">
                  {step.kind === "email" ? step.subject : step.task_name}
                </span>
                <span className="text-xs text-muted-foreground">
                  {delivery?.status?.replaceAll("_", " ") ??
                    (i === row.step_index ? labels[row.status] : "Not started")}
                </span>
              </div>
              {delivery?.error && (
                <p className="mt-2 text-xs text-destructive">
                  {delivery.error}
                </p>
              )}
              {step.kind === "task" && delivery?.result_id && slug && (
                <a
                  className="mt-2 inline-block text-xs underline"
                  href={`/w/${slug}/pm/tasks/${delivery.result_id}`}
                >
                  Open task
                </a>
              )}
              {step.kind === "email" && delivery?.status === "sent" && (
                <details className="mt-2 text-xs text-muted-foreground">
                  <summary className="cursor-pointer">
                    View sent email ·{" "}
                    {format(new Date(delivery.updated_at), "MMM d, h:mm a")}
                  </summary>
                  <EmailBodyRenderer
                    html={delivery.body_html}
                    collapsedByDefault={false}
                  />
                </details>
              )}
            </div>
          );
        })}
      </div>
    </>
  );
}
