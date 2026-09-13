import { useState } from "react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { QuietUnderlineInput } from "@/components/design-system/quiet";
import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import { EmailContentEditor } from "./EmailContentEditor";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import { useOutreachRefresh } from "@/hooks/queries/useCRMOutreach";
import type { EmailTemplate } from "@/lib/crmOutreachTypes";
export function EmailTemplateEditor({
  workspaceId,
  initial,
  onClose,
}: {
  workspaceId: string;
  initial?: Partial<EmailTemplate>;
  onClose: () => void;
}) {
  const [form, setForm] = useState({
    name: initial?.name ?? "",
    subject: initial?.subject ?? "",
    body_html: initial?.body_html ?? "<p>Hi {{first_name|there}},</p><p></p>",
    shared: initial?.shared ?? true,
  });
  const [saving, setSaving] = useState(false);
  const [discard, setDiscard] = useState(false);
  const [changed, setChanged] = useState(false);
  const refresh = useOutreachRefresh(workspaceId);
  const patch = (changes: Partial<typeof form>) => {
    setForm((current) => ({ ...current, ...changes }));
    setChanged(true);
  };
  const close = () => {
    if (saving) return;
    if (changed) setDiscard(true);
    else onClose();
  };
  return (
    <>
      <Dialog open onOpenChange={(open) => !open && close()}>
        <DialogContent
          className="max-h-[90vh] overflow-y-auto sm:max-w-2xl"
          aria-describedby={undefined}
        >
          <DialogHeader>
            <DialogTitle>
              {initial?.id ? "Edit template" : "New template"}
            </DialogTitle>
          </DialogHeader>
          <label className="text-xs text-muted-foreground">
            Name
            <QuietUnderlineInput
              aria-label="Template name"
              value={form.name}
              onChange={(event) => patch({ name: event.target.value })}
              placeholder="e.g. After a demo"
              disabled={saving}
            />
          </label>
          <label className="text-xs text-muted-foreground">
            Subject
            <QuietUnderlineInput
              aria-label="Template subject"
              value={form.subject}
              onChange={(event) => patch({ subject: event.target.value })}
              placeholder="Email subject"
              disabled={saving}
            />
          </label>
          <EmailContentEditor
            workspaceId={workspaceId}
            value={form.body_html}
            onChange={(body_html) => patch({ body_html })}
            disabled={saving}
          />
          <div className="flex items-center justify-between gap-3">
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              <input
                type="checkbox"
                checked={form.shared}
                onChange={(event) => patch({ shared: event.target.checked })}
                disabled={saving}
              />
              Share with CRM members
            </label>
            <Button
              disabled={saving || !form.name.trim() || !form.subject.trim()}
              onClick={async () => {
                setSaving(true);
                try {
                  unwrap(
                    await crmOutreachService.saveTemplate(workspaceId, {
                      ...initial,
                      ...form,
                    }),
                  );
                  await refresh();
                  toast.success("Template saved");
                  onClose();
                } catch (error) {
                  toast.error(
                    error instanceof Error
                      ? error.message
                      : "Could not save template",
                  );
                } finally {
                  setSaving(false);
                }
              }}
            >
              {saving ? "Saving…" : "Save template"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={discard}
        onOpenChange={setDiscard}
        title="Discard template changes?"
        description="Your changes haven’t been saved."
        confirmLabel="Discard changes"
        cancelLabel="Keep editing"
        onConfirm={onClose}
      />
    </>
  );
}
