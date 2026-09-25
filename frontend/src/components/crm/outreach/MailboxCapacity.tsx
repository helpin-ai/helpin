import { useState } from "react";
import { format } from "date-fns";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { QuickTooltip } from "@/components/ui/quick-tooltip";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from "@/components/design-system/quiet";
import { useWorkspaceAccess, usePermissions } from "@/hooks/queries/useSession";
import {
  useMailboxCapacity,
  useOutreachRefresh,
} from "@/hooks/queries/useCRMOutreach";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import type {
  MailboxCapacity,
  MailboxSendingPolicy,
} from "@/lib/crmOutreachTypes";

export function MailboxCapacityHint({
  workspaceId,
  accountId,
}: {
  workspaceId: string;
  accountId: string;
}) {
  const access = useWorkspaceAccess(workspaceId);
  const query = useMailboxCapacity(
    workspaceId,
    usePermissions(access.data).has("crm.edit"),
  );
  const row = query.data?.find((item) => item.account_id === accountId);
  if (!row) return null;
  return (
    <p className="text-xs text-muted-foreground">
      {row.sequence_remaining} sequence emails available · Extra recipients stay
      queued.
    </p>
  );
}

export function MailboxCapacityPanel({
  workspaceId,
  accountId,
}: {
  workspaceId: string;
  accountId?: string;
}) {
  const access = useWorkspaceAccess(workspaceId);
  const canEdit = usePermissions(access.data).has("crm.edit");
  const query = useMailboxCapacity(workspaceId, canEdit);
  const [editing, setEditing] = useState<MailboxCapacity>();
  const slug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  if (!canEdit) return null;
  const rows = (query.data ?? []).filter(
    (row) => !accountId || row.account_id === accountId,
  );
  if (query.isError)
    return (
      <Button variant="ghost" size="sm" onClick={() => void query.refetch()}>
        Couldn’t load sending capacity. Try again
      </Button>
    );
  if (!rows.length) {
    if (query.isPending || accountId || !slug) return null;
    return (
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-quiet-border-soft py-3 text-xs text-muted-foreground">
        <span>Connect Gmail to view sending limits and capacity.</span>
        <QuietTextAction asChild>
          <a href={`/w/${slug}/settings/crm-email`}>Connect a sending mailbox</a>
        </QuietTextAction>
      </div>
    );
  }
  return (
    <div className="space-y-2">
      {rows.map((row) => (
        <div
          key={row.account_id}
          className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 border-b border-quiet-border-soft py-3 text-xs"
        >
          <div className="min-w-0 flex-1 basis-44">
            <p className="truncate font-medium">
              {accountId ? "Sending capacity" : row.email}
            </p>
            <QuickTooltip label="All Helpin CRM sends and pending attempts across this mailbox’s workspaces in the last 24 hours. Emails sent directly from Gmail are not counted.">
              <div
                tabIndex={0}
                className="mt-1 flex cursor-help items-center gap-2 text-muted-foreground"
              >
                <span
                  className="h-1 w-16 overflow-hidden rounded-full bg-muted"
                  aria-hidden="true"
                >
                  <span
                    className="block h-full rounded-full bg-primary"
                    style={{
                      width: `${Math.min(100, (row.used / row.daily_limit) * 100)}%`,
                    }}
                  />
                </span>
                <span>
                  {row.used} / {row.daily_limit} used
                </span>
              </div>
            </QuickTooltip>
          </div>
          <div className="text-muted-foreground">
            <span>{row.queued} queued</span>
            {row.next_available_at && (
              <p className="mt-1 text-[11px]">
                Next capacity ·{" "}
                {format(new Date(row.next_available_at), "MMM d, h:mm a")}
              </p>
            )}
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs"
            aria-label={`Sending limits for ${row.email}`}
            onClick={() => setEditing(row)}
          >
            Sending limits
          </Button>
        </div>
      ))}
      {editing && (
        <SendingLimits
          workspaceId={workspaceId}
          row={editing}
          onClose={() => setEditing(undefined)}
        />
      )}
    </div>
  );
}

function SendingLimits({
  workspaceId,
  row,
  onClose,
}: {
  workspaceId: string;
  row: MailboxCapacity;
  onClose: () => void;
}) {
  const [form, setForm] = useState<MailboxSendingPolicy>({
    daily_limit: row.daily_limit,
    manual_reserve: row.manual_reserve,
    min_interval_seconds: row.min_interval_seconds,
  });
  const [busy, setBusy] = useState(false);
  const refresh = useOutreachRefresh(workspaceId);
  const valid =
    Number.isInteger(form.daily_limit) &&
    form.daily_limit >= 1 &&
    form.daily_limit <= 1000 &&
    Number.isInteger(form.manual_reserve) &&
    form.manual_reserve >= 0 &&
    form.manual_reserve < form.daily_limit &&
    Number.isInteger(form.min_interval_seconds) &&
    form.min_interval_seconds >= 60 &&
    form.min_interval_seconds <= 3600;
  const fields = [
    {
      key: "daily_limit" as const,
      label: "Daily email limit",
      min: 1,
      max: 1000,
      tip: "All Helpin CRM emails per rolling 24 hours, shared across workspaces. Your provider may enforce a lower limit.",
    },
    {
      key: "manual_reserve" as const,
      label: "Reserved for manual emails",
      min: 0,
      max: form.daily_limit - 1,
      tip: "Keeps part of your daily budget available for manual emails and replies. Sequences use the remaining budget.",
    },
    {
      key: "min_interval_seconds" as const,
      label: "Time between sequence emails (seconds)",
      min: 60,
      max: 3600,
      tip: "Minimum spacing for automated sends. Manual emails and replies can send immediately when capacity is available.",
    },
  ];
  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Sending limits</DialogTitle>
          <DialogDescription className="break-all">
            {row.email}
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-5"
          onSubmit={async (event) => {
            event.preventDefault();
            if (!valid || busy) return;
            setBusy(true);
            try {
              unwrap(
                await crmOutreachService.saveMailboxCapacity(
                  workspaceId,
                  row.account_id,
                  form,
                ),
              );
              await refresh();
              toast.success("Sending limits saved");
              onClose();
            } catch (error) {
              toast.error(
                error instanceof Error
                  ? error.message
                  : "Could not save sending limits",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          {fields.map((field) => (
            <label
              key={field.key}
              className="block space-y-1 text-xs text-muted-foreground"
            >
              <QuickTooltip label={field.tip}>
                <span
                  tabIndex={0}
                  className="cursor-help border-b border-dotted border-muted-foreground/40"
                >
                  {field.label}
                </span>
              </QuickTooltip>
              <QuietUnderlineInput
                aria-label={field.label}
                type="number"
                required
                min={field.min}
                max={field.max}
                value={form[field.key]}
                disabled={busy}
                onChange={(event) =>
                  setForm({ ...form, [field.key]: Number(event.target.value) })
                }
              />
            </label>
          ))}
          <p className="text-xs text-muted-foreground">
            Up to {Math.max(0, form.daily_limit - form.manual_reserve)} sequence
            emails per 24 hours. Follow-ups take priority; extra emails stay
            queued.
          </p>
          <div className="flex justify-end gap-2">
            <QuietTextAction
              type="button"
              disabled={busy}
              onClick={onClose}
            >
              Cancel
            </QuietTextAction>
            <QuietPrimaryAction type="submit" disabled={busy || !valid}>
              {busy ? "Saving…" : "Save limits"}
            </QuietPrimaryAction>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
