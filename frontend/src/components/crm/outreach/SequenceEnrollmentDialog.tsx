import { MailboxCapacityHint } from "./MailboxCapacity";
import { useState } from "react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  QuietSearchInput,
  QuietSelect,
} from "@/components/design-system/quiet";
import { useContacts, useEmailAccounts } from "@/hooks/queries/useCRM";
import {
  useEmailSequences,
  useOutreachRefresh,
} from "@/hooks/queries/useCRMOutreach";
import { useAuthStore } from "@/stores/authStore";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import type {
  EnrollmentPreview,
  EnrollmentRequest,
} from "@/lib/crmOutreachTypes";
import { EmailBodyRenderer } from "@/components/support/EmailBodyRenderer";
export function SequenceEnrollmentDialog({
  workspaceId,
  sequenceId,
  contactIds = [],
  dealId,
  onClose,
}: {
  workspaceId: string;
  sequenceId?: string;
  contactIds?: string[];
  dealId?: string;
  onClose: () => void;
}) {
  const user = useAuthStore((s) => s.user?.id);
  const sequences = useEmailSequences(workspaceId);
  const accounts = useEmailAccounts(workspaceId);
  const refresh = useOutreachRefresh(workspaceId);
  const [sequence, setSequence] = useState(sequenceId ?? "");
  const [account, setAccount] = useState("");
  const [selected, setSelected] = useState(contactIds);
  const [search, setSearch] = useState("");
  const [preview, setPreview] = useState<EnrollmentPreview[] | null>(null);
  const [request, setRequest] = useState<EnrollmentRequest | null>(null);
  const [busy, setBusy] = useState(false);
  const [index, setIndex] = useState(0);
  const contacts = useContacts(workspaceId, { search, per_page: 30 });
  const active = (sequences.data ?? []).filter(
    (row) => row.status === "active",
  );
  const seq = active.find((row) => row.id === (sequence || active[0]?.id));
  const mailboxes = (accounts.data ?? []).filter(
    (row) =>
      row.member_id === user &&
      row.can_send &&
      row.status === "connected" &&
      row.is_active,
  );
  const sender = mailboxes.find(
    (row) => row.id === (account || mailboxes[0]?.id),
  );
  const person = preview?.[index];
  const blocked = preview?.some((row) => row.error);
  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent
        className="max-h-[92vh] overflow-y-auto sm:max-w-3xl"
        aria-describedby="enrollment-description"
      >
        <DialogHeader>
          <DialogTitle>
            {preview ? "Review enrollment" : "Add to sequence"}
          </DialogTitle>
          <DialogDescription id="enrollment-description">
            {preview
              ? `${preview.length} recipient${preview.length === 1 ? "" : "s"} · ${sender?.email_address}`
              : "Choose the sequence and the mailbox that will send the emails."}
          </DialogDescription>
        </DialogHeader>
        {!preview ? (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-xs text-muted-foreground">
                Sequence
                <QuietSelect
                  label="Sequence"
                  value={seq?.id ?? ""}
                  onChange={setSequence}
                  options={active.map((row) => ({
                    value: row.id,
                    label: row.name,
                  }))}
                />
              </label>
              <label className="text-xs text-muted-foreground">
                From
                <QuietSelect
                  label="Sending mailbox"
                  value={sender?.id ?? ""}
                  onChange={setAccount}
                  options={mailboxes.map((row) => ({
                    value: row.id,
                    label: row.email_address,
                  }))}
                />
              </label>
            </div>
            {sender && (
              <MailboxCapacityHint
                workspaceId={workspaceId}
                accountId={sender.id}
              />
            )}
            {sequences.isLoading || accounts.isLoading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : !seq ? (
              <p className="text-sm text-muted-foreground">
                Publish a sequence in CRM → Emails to get started.
              </p>
            ) : !sender ? (
              <p className="text-sm text-muted-foreground">
                Connect your Gmail account in Settings → CRM Email first.
              </p>
            ) : null}
            {!contactIds.length && !dealId ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Contacts</span>
                  <span className="text-xs text-muted-foreground">
                    {selected.length} selected{" "}
                    {selected.length > 0 && (
                      <button
                        className="ml-2 underline"
                        onClick={() => setSelected([])}
                      >
                        Clear
                      </button>
                    )}
                  </span>
                </div>
                <QuietSearchInput
                  placeholder="Search contacts"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
                <div className="max-h-60 overflow-y-auto divide-y divide-border/40">
                  {contacts.isLoading ? (
                    <p className="py-4 text-xs text-muted-foreground">
                      Loading contacts…
                    </p>
                  ) : contacts.isError ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => void contacts.refetch()}
                    >
                      Couldn’t load contacts. Try again
                    </Button>
                  ) : !contacts.data?.data.length ? (
                    <p className="py-4 text-xs text-muted-foreground">
                      {search ? "No matching contacts" : "No contacts yet"}
                    </p>
                  ) : null}
                  {(contacts.data?.data ?? []).map((contact) => (
                    <label
                      key={contact.id}
                      className="flex cursor-pointer items-center gap-3 px-1 py-2.5 text-sm"
                    >
                      <input
                        type="checkbox"
                        disabled={
                          !contact.email ||
                          (selected.length >= 100 &&
                            !selected.includes(contact.id))
                        }
                        checked={selected.includes(contact.id)}
                        onChange={(event) =>
                          setSelected((current) =>
                            event.target.checked
                              ? [...current, contact.id]
                              : current.filter((id) => id !== contact.id),
                          )
                        }
                      />
                      <span className="min-w-0">
                        <span className="block truncate">
                          {contact.first_name} {contact.last_name}
                        </span>
                        <span className="block truncate text-xs text-muted-foreground">
                          {contact.email || "No email address"}
                        </span>
                      </span>
                    </label>
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                {dealId
                  ? "Linked contacts will be checked before enrollment."
                  : `${contactIds.length} contact selected`}
              </p>
            )}
            {seq && (
              <p className="text-xs text-muted-foreground">
                {seq.steps.length} steps ·{" "}
                {seq.weekdays ? "Weekdays" : "Every day"} · {seq.start_hour}:00–
                {seq.end_hour}:00 ({seq.timezone})
              </p>
            )}
            <div className="flex justify-end">
              <Button
                disabled={
                  busy || !seq || !sender || (!selected.length && !dealId)
                }
                onClick={async () => {
                  if (!seq || !sender) return;
                  setBusy(true);
                  try {
                    const req = {
                      account_id: sender.id,
                      contact_ids: selected,
                      deal_id: dealId,
                      version: seq.version,
                    };
                    const rows = unwrap(
                      await crmOutreachService.preview(
                        workspaceId,
                        seq.id,
                        req,
                      ),
                    );
                    setRequest(req);
                    setPreview(rows);
                    setIndex(0);
                  } catch (error) {
                    toast.error(
                      error instanceof Error
                        ? error.message
                        : "Could not preview enrollment",
                    );
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {busy ? "Preparing…" : "Review emails"}
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid min-w-0 gap-4 sm:grid-cols-[190px_minmax(0,1fr)]">
              <div className="max-h-72 space-y-1 overflow-auto">
                {preview.map((row, i) => (
                  <button
                    key={row.contact_id}
                    onClick={() => setIndex(i)}
                    className={`w-full rounded-sm px-2 py-2 text-left text-sm ${i === index ? "bg-muted/50" : ""}`}
                  >
                    <span className="block truncate font-medium">
                      {row.contact_name}
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {row.email}
                    </span>
                    {row.error && (
                      <span className="mt-1 block text-xs text-destructive">
                        {row.error}
                      </span>
                    )}
                  </button>
                ))}
              </div>
              <div className="min-w-0 max-h-[50vh] space-y-3 overflow-y-auto">
                {person?.steps.map((step, i) => (
                  <details
                    key={i}
                    open={i === 0}
                    className="border-b border-border/50 pb-3"
                  >
                    <summary className="cursor-pointer text-sm font-medium">
                      {i + 1}.{" "}
                      {step.kind === "task" ? step.task_name : step.subject}
                      <span className="ml-2 text-xs font-normal text-muted-foreground">
                        {step.delay_days
                          ? `Wait ${step.delay_days}d`
                          : "No delay"}
                        {step.mode === "review"
                          ? " · Review before sending"
                          : ""}
                      </span>
                    </summary>
                    {step.kind === "email" && (
                      <div className="mt-2">
                        <EmailBodyRenderer
                          html={step.body_html ?? ""}
                          collapsedByDefault={false}
                          constrainHeight={false}
                        />
                      </div>
                    )}
                  </details>
                ))}
              </div>
            </div>
            {blocked && (
              <p role="alert" className="text-sm text-destructive">
                Resolve the issues shown for each contact before starting.
              </p>
            )}
            <p className="text-xs text-muted-foreground">
              Stops on replies, undeliverable addresses, unsubscribe, or a
              closed deal. Each email includes an unsubscribe link.
            </p>
            <div className="flex justify-between">
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => setPreview(null)}
              >
                Back
              </Button>
              <Button
                disabled={busy || blocked || !preview.length}
                onClick={async () => {
                  if (!seq || !request) return;
                  setBusy(true);
                  try {
                    unwrap(
                      await crmOutreachService.enroll(
                        workspaceId,
                        seq.id,
                        request,
                      ),
                    );
                    await refresh();
                    toast.success(
                      `${preview.length} recipient${preview.length === 1 ? "" : "s"} enrolled`,
                    );
                    onClose();
                  } catch (error) {
                    toast.error(
                      error instanceof Error
                        ? error.message
                        : "Could not enroll contacts",
                    );
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {busy ? "Starting…" : `Start sequence for ${preview.length}`}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
