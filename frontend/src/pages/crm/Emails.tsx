import { EmailStarterLibrary } from "@/components/crm/outreach/EmailStarterLibrary";
import { useWorkspaceAccess, usePermissions } from "@/hooks/queries/useSession";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  QuietPageHeader,
  QuietSearchInput,
  QuietEmptyState,
  QuietSelect,
} from "@/components/design-system/quiet";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useAuthStore } from "@/stores/authStore";
import {
  useEmailTemplates,
  useEmailSequences,
  useOutreachRefresh,
} from "@/hooks/queries/useCRMOutreach";
import { EmailTemplateEditor } from "@/components/crm/outreach/EmailTemplateEditor";
import { SequenceEditor } from "@/components/crm/outreach/SequenceEditor";
import { SequenceActivity } from "@/components/crm/outreach/SequenceActivity";
import { SequenceEnrollmentDialog } from "@/components/crm/outreach/SequenceEnrollmentDialog";
import { EmailBodyRenderer } from "@/components/support/EmailBodyRenderer";
import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import type { EmailTemplate, EmailSequence } from "@/lib/crmOutreachTypes";

export function EmailsPage() {
  const ws = useWorkspaceStore((s) => s.currentWorkspace);
  const user = useAuthStore((s) => s.user?.id);
  const workspaceId = ws?.id ?? "";
  const access = useWorkspaceAccess(workspaceId);
  const canEdit = usePermissions(access.data).has("crm.edit");
  const templates = useEmailTemplates(workspaceId);
  const sequences = useEmailSequences(workspaceId);
  const refresh = useOutreachRefresh(workspaceId);
  const [tab, setTab] = useState<"templates" | "sequences" | "activity">(
    new URLSearchParams(window.location.search).get("tab") === "sequences"
      ? "sequences"
      : "templates",
  );
  const [search, setSearch] = useState("");
  const [scope, setScope] = useState("all");
  const [template, setTemplate] = useState<Partial<EmailTemplate>>();
  const [preview, setPreview] = useState<EmailTemplate>();
  const [sequence, setSequence] = useState<Partial<EmailSequence>>();
  const [remove, setRemove] = useState<EmailTemplate>();
  const [starters, setStarters] = useState<"templates" | "sequences">();
  const [enroll, setEnroll] = useState(false);
  if (!workspaceId) return null;
  if (sequence)
    return (
      <SequenceEditor
        workspaceId={workspaceId}
        initial={sequence}
        onSaved={setSequence}
        onClose={() => {
          setSequence(undefined);
          setTab("sequences");
        }}
      />
    );
  const filteredTemplates = (templates.data ?? []).filter(
    (t) =>
      `${t.name} ${t.subject}`.toLowerCase().includes(search.toLowerCase()) &&
      (scope === "all" || (scope === "mine" ? t.owner_id === user : t.shared)),
  );
  const filteredSequences = (sequences.data ?? []).filter(
    (s) =>
      s.status !== "archived" &&
      s.name.toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <div className="flex h-full min-w-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="Emails"
        actions={
          <>
            {canEdit && tab !== "activity" && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setStarters(tab)}
              >
                Browse starters
              </Button>
            )}
            {canEdit && tab === "templates" && (
              <Button size="sm" onClick={() => setTemplate({})}>
                New template
              </Button>
            )}
            {canEdit && tab === "sequences" && (
              <Button size="sm" onClick={() => setSequence({})}>
                New sequence
              </Button>
            )}
            {canEdit && tab === "activity" && (
              <Button size="sm" onClick={() => setEnroll(true)}>
                Add contacts
              </Button>
            )}
          </>
        }
      />
      <div className="flex gap-6 border-b border-border/50 px-6 text-sm">
        {(["templates", "sequences", "activity"] as const).map((value) => (
          <button
            key={value}
            onClick={() => {
              setTab(value);
              setSearch("");
            }}
            className={`py-3 capitalize ${tab === value ? "border-b-2 border-foreground font-medium" : "text-muted-foreground"}`}
          >
            {value}
          </button>
        ))}
      </div>
      <div className="min-h-0 flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto max-w-6xl space-y-5">
          {tab === "activity" ? (
            <SequenceActivity workspaceId={workspaceId} />
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-3">
                <QuietSearchInput
                  containerClassName="min-w-0 flex-1"
                  placeholder={`Search ${tab}`}
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
                {canEdit && tab === "templates" && (
                  <QuietSelect
                    label="Template visibility"
                    value={scope}
                    onChange={setScope}
                    options={[
                      { value: "all", label: "All templates" },
                      { value: "mine", label: "Created by me" },
                      { value: "shared", label: "Shared" },
                    ]}
                  />
                )}
              </div>
              {(tab === "templates" ? templates : sequences).isLoading ? (
                <p className="py-8 text-sm text-muted-foreground">
                  Loading {tab}…
                </p>
              ) : (tab === "templates" ? templates : sequences).isError ? (
                <Button
                  variant="ghost"
                  onClick={() =>
                    void (tab === "templates" ? templates : sequences).refetch()
                  }
                >
                  Couldn’t load {tab}. Try again
                </Button>
              ) : tab === "templates" ? (
                filteredTemplates.length ? (
                  <div className="divide-y divide-border/50">
                    {filteredTemplates.map((row) => (
                      <div
                        key={row.id}
                        className="group flex items-center gap-3 py-4"
                      >
                        <button
                          className="min-w-0 flex-1 text-left"
                          onClick={() => setPreview(row)}
                        >
                          <span className="block truncate text-sm font-medium">
                            {row.name}
                          </span>
                          <span className="mt-1 block truncate text-xs text-muted-foreground">
                            {row.subject}
                          </span>
                        </button>
                        <span className="text-xs text-muted-foreground">
                          {row.shared ? "Shared" : "Private"}
                        </span>
                        {row.owner_id === user && canEdit && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => setTemplate(row)}
                          >
                            Edit
                          </Button>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <QuietEmptyState
                    title={
                      search
                        ? "No matching templates"
                        : "Your best emails, ready to reuse"
                    }
                    description={
                      search
                        ? "Try another search."
                        : "Save a message once. Personalize it in any CRM email or sequence."
                    }
                    action={
                      canEdit && (
                        <Button size="sm" onClick={() => setTemplate({})}>
                          Create template
                        </Button>
                      )
                    }
                  />
                )
              ) : filteredSequences.length ? (
                <div className="divide-y divide-border/50">
                  {filteredSequences.map((row) => (
                    <button
                      key={row.id}
                      onClick={() => setSequence(row)}
                      className="flex w-full items-center gap-3 py-4 text-left hover:bg-muted/20"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">
                          {row.name}
                        </span>
                        <span className="mt-1 block text-xs text-muted-foreground">
                          {row.steps.length} steps ·{" "}
                          {row.entry_stage_id
                            ? "Stage enrollment"
                            : "Manual enrollment"}
                        </span>
                      </span>
                      <span className="text-xs capitalize text-muted-foreground">
                        {row.status === "active" ? "Published" : row.status}
                      </span>
                      <span aria-hidden className="text-muted-foreground">
                        →
                      </span>
                    </button>
                  ))}
                </div>
              ) : (
                <QuietEmptyState
                  title={
                    search
                      ? "No matching sequences"
                      : "Make every follow-up count"
                  }
                  description={
                    search
                      ? "Try another search."
                      : "Combine emails and tasks, with time for a reply between each step."
                  }
                  action={
                    canEdit && (
                      <Button size="sm" onClick={() => setSequence({})}>
                        Create sequence
                      </Button>
                    )
                  }
                />
              )}
            </>
          )}
        </div>
      </div>
      {starters && (
        <EmailStarterLibrary
          mode={starters}
          onClose={() => setStarters(undefined)}
          onTemplate={setTemplate}
          onSequence={setSequence}
        />
      )}
      {template && (
        <EmailTemplateEditor
          workspaceId={workspaceId}
          initial={template}
          onClose={() => setTemplate(undefined)}
        />
      )}
      {preview && (
        <Dialog open onOpenChange={(open) => !open && setPreview(undefined)}>
          <DialogContent
            className="max-h-[90vh] overflow-auto sm:max-w-2xl"
            aria-describedby={undefined}
          >
            <DialogHeader>
              <DialogTitle>{preview.name}</DialogTitle>
            </DialogHeader>
            <p className="border-b border-border/50 pb-3 text-sm font-medium">
              {preview.subject}
            </p>
            <EmailBodyRenderer
              html={preview.body_html}
              collapsedByDefault={false}
            />
            <div className="flex justify-end gap-2">
              <Button
                disabled={!canEdit}
                variant="ghost"
                onClick={() => {
                  setTemplate({
                    name: `${preview.name} (copy)`,
                    subject: preview.subject,
                    body_html: preview.body_html,
                    shared: false,
                  });
                  setPreview(undefined);
                }}
              >
                Duplicate
              </Button>
              {preview.owner_id === user && canEdit && (
                <>
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setRemove(preview);
                      setPreview(undefined);
                    }}
                  >
                    Delete
                  </Button>
                  <Button
                    onClick={() => {
                      setTemplate(preview);
                      setPreview(undefined);
                    }}
                  >
                    Edit template
                  </Button>
                </>
              )}
            </div>
          </DialogContent>
        </Dialog>
      )}
      <ConfirmDialog
        open={Boolean(remove)}
        onOpenChange={(open) => !open && setRemove(undefined)}
        title="Delete template?"
        description="Existing emails and sequence steps will keep their content."
        confirmLabel="Delete template"
        onConfirm={async () => {
          if (!remove) return;
          try {
            unwrap(
              await crmOutreachService.deleteTemplate(workspaceId, remove.id),
            );
            await refresh();
            setRemove(undefined);
          } catch (error) {
            toast.error(
              error instanceof Error
                ? error.message
                : "Could not delete template",
            );
          }
        }}
      />
      {enroll && (
        <SequenceEnrollmentDialog
          workspaceId={workspaceId}
          onClose={() => setEnroll(false)}
        />
      )}
    </div>
  );
}
