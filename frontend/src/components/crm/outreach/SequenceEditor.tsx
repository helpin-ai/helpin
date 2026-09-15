import { QuickTooltip } from "@/components/ui/quick-tooltip";
import { ChevronRightIcon } from "@/lib/pmIcons";
import { useWorkspaceAccess, usePermissions } from "@/hooks/queries/useSession";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  QuietPageHeader,
  QuietUnderlineInput,
  QuietSelect,
  QuietTextAction,
} from "@/components/design-system/quiet";
import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import { useAccessibleTeams } from "@/hooks/useAccessibleTeams";
import { useEmailAccounts, usePipelines } from "@/hooks/queries/useCRM";
import { useOutreachRefresh } from "@/hooks/queries/useCRMOutreach";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useAuthStore } from "@/stores/authStore";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { unwrap } from "@/lib/queryUtils";
import type { EmailSequence, SequenceStep } from "@/lib/crmOutreachTypes";
import { EmailContentEditor } from "./EmailContentEditor";
import { EmailTemplatePicker } from "./EmailTemplatePicker";
import { SequenceEnrollmentDialog } from "./SequenceEnrollmentDialog";
import { SequenceActivity } from "./SequenceActivity";

export function SequenceEditor({
  workspaceId,
  initial,
  onClose,
  onSaved,
}: {
  workspaceId: string;
  initial: Partial<EmailSequence>;
  onClose: () => void;
  onSaved: (sequence: EmailSequence) => void;
}) {
  const [form, setForm] = useState<Partial<EmailSequence>>({
    name: "New sequence",
    status: "draft",
    version: 0,
    steps: [
      {
        kind: "email",
        mode: "automatic",
        delay_days: 0,
        subject: "",
        body_html: "<p>Hi {{first_name|there}},</p><p></p>",
      },
    ],
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    start_hour: 9,
    end_hour: 17,
    weekdays: true,
    include_signature: true,
    daily_new_recipients: 25,
    entry_stage_id: "",
    entry_account_id: "",
    ...initial,
  });
  const [active, setActive] = useState(0);
  const [dirty, setDirty] = useState(Boolean(initial.name && !initial.id));
  const [saving, setSaving] = useState(false);
  const [discard, setDiscard] = useState(false);
  const [enroll, setEnroll] = useState(false);
  const [tab, setTab] = useState<"steps" | "activity">("steps");
  const refresh = useOutreachRefresh(workspaceId);
  const teams = useAccessibleTeams(workspaceId);
  const accounts = useEmailAccounts(workspaceId);
  const pipelines = usePipelines(workspaceId);
  const [chosenPipeline, setChosenPipeline] = useState<string>();
  const user = useAuthStore((s) => s.user?.id);
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug);
  const access = useWorkspaceAccess(workspaceId);
  const canEdit = usePermissions(access.data).has("crm.edit");
  const editable = canEdit && (!initial.owner_id || initial.owner_id === user);
  const steps = form.steps ?? [];
  const mailboxes = (accounts.data ?? []).filter(
    (a) =>
      a.member_id === user &&
      a.can_send &&
      a.is_active &&
      a.status === "connected",
  );
  const pipelineId =
    chosenPipeline ??
    (pipelines.data ?? []).find((pipeline) =>
      pipeline.stages?.some((stage) => stage.id === form.entry_stage_id),
    )?.id ??
    "manual";
  const stages = (
    (pipelines.data ?? []).find((pipeline) => pipeline.id === pipelineId)
      ?.stages ?? []
  )
    .filter((stage) => !["won", "lost"].includes(stage.stage_type))
    .map((stage) => ({ value: stage.id, label: stage.name }));
  const needsStage = pipelineId !== "manual" && !form.entry_stage_id;
  const patch = (change: Partial<EmailSequence>) => {
    setForm((current) => ({ ...current, ...change }));
    setDirty(true);
  };
  const patchStep = (index: number, change: Partial<SequenceStep>) =>
    patch({
      steps: steps.map((step, i) =>
        i === index ? { ...step, ...change } : step,
      ),
    });
  const save = async (status = form.status) => {
    if (needsStage && status !== "archived") {
      toast.error("Choose a stage for automatic enrollment.");
      return;
    }
    setSaving(true);
    try {
      const row = unwrap(
        await crmOutreachService.saveSequence(workspaceId, { ...form, status }),
      );
      if (!row?.id) {
        throw new Error(
          "Could not confirm the saved sequence. Your changes are still in the editor.",
        );
      }
      setForm(row);
      setDirty(false);
      await refresh();
      onSaved(row);
      toast.success(
        status === "active" ? "Sequence published" : "Sequence saved",
      );
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Could not save sequence",
      );
    } finally {
      setSaving(false);
    }
  };
  const close = () => (dirty ? setDiscard(true) : onClose());
  const add = (kind: "email" | "task") => {
    patch({
      steps: [
        ...steps,
        {
          kind,
          mode: kind === "email" ? "automatic" : undefined,
          delay_days: steps.length ? 2 : 0,
          subject: "",
          body_html: "<p></p>",
          task_name:
            kind === "task" ? "Follow up with {{full_name}}" : undefined,
          team_id: kind === "task" ? teams.teams[0]?.id : undefined,
        },
      ],
    });
    setActive(steps.length);
  };
  return (
    <div className="flex h-full min-w-0 flex-col">
      <QuietPageHeader
        variant="shell"
        className="[&>div:last-child]:flex-wrap [&>div:last-child>div:last-child]:max-w-full"
        title={
          <QuietUnderlineInput
            aria-label="Sequence name"
            value={form.name}
            onChange={(event) => patch({ name: event.target.value })}
            disabled={!editable || saving}
            className="w-72 max-w-full border-b-transparent text-xl md:text-xl font-semibold hover:border-border focus:border-border"
          />
        }
        navigation={
          <QuietTextAction onClick={close}>← Sequences</QuietTextAction>
        }
        actions={
          <>
            {editable && dirty && (
              <Button
                variant="ghost"
                size="sm"
                disabled={saving || !dirty || needsStage}
                onClick={() => void save()}
              >
                Save {form.status === "draft" ? "draft" : "changes"}
              </Button>
            )}
            {editable && (
              <Button
                size="sm"
                variant={form.status === "active" ? "ghost" : "default"}
                disabled={saving}
                onClick={() =>
                  void save(form.status === "active" ? "paused" : "active")
                }
              >
                {saving
                  ? "Saving…"
                  : form.status === "active"
                    ? "Pause sequence"
                    : "Publish sequence"}
              </Button>
            )}
            {canEdit && !dirty && form.id && form.status === "active" && (
              <Button
                size="sm"
                disabled={dirty}
                onClick={() => setEnroll(true)}
              >
                Add contacts
              </Button>
            )}
          </>
        }
      />
      <div className="flex gap-5 border-b border-border/50 px-6 text-sm">
        {(["steps", "activity"] as const).map((value) => (
          <button
            key={value}
            onClick={() => setTab(value)}
            className={`py-3 capitalize ${tab === value ? "border-b-2 border-foreground font-medium" : "text-muted-foreground"}`}
            disabled={value === "activity" && !form.id}
          >
            {value}
          </button>
        ))}
      </div>
      {tab === "activity" && form.id ? (
        <div className="min-h-0 flex-1 overflow-auto p-6">
          <SequenceActivity workspaceId={workspaceId} sequenceId={form.id} />
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto">
          <div className="mx-auto grid max-w-6xl gap-8 px-4 py-6 sm:px-6 lg:grid-cols-[minmax(0,1fr)_260px]">
            <main className="min-w-0 space-y-3">
              {steps.map((step, index) => (
                <section
                  key={index}
                  className={`min-w-0 rounded-lg border ${active === index ? "border-border" : "border-border/50"}`}
                >
                  <div className="flex items-center gap-2 px-4 py-3">
                    <button
                      onClick={() => setActive(active === index ? -1 : index)}
                      aria-expanded={active === index}
                      className="flex min-w-0 flex-1 items-center gap-2 rounded-md text-left text-sm hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <ChevronRightIcon
                        aria-hidden="true"
                        className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-150 motion-reduce:transition-none ${active === index ? "rotate-90" : ""}`}
                      />
                      <span className="text-xs text-muted-foreground">
                        {index + 1}
                      </span>
                      <span className="min-w-0 truncate font-medium">
                        {step.kind === "email"
                          ? active === index
                            ? "Email"
                            : step.subject || "Email"
                          : active === index
                            ? "Follow-up task"
                            : step.task_name || "Follow-up task"}
                      </span>
                      <span className="ml-2 text-xs text-muted-foreground">
                        {step.delay_days
                          ? `Wait ${step.delay_days}d`
                          : index === 0
                            ? "Start immediately"
                            : "No delay"}
                      </span>
                    </button>
                    {editable && (
                      <div className="flex shrink-0 gap-1">
                        {[-1, 1].map((direction) => (
                          <Button
                            key={direction}
                            variant="ghost"
                            size="sm"
                            aria-label={`Move step ${index + 1} ${direction < 0 ? "up" : "down"}`}
                            className="h-6 w-6 p-0"
                            disabled={
                              saving ||
                              index + direction < 0 ||
                              index + direction >= steps.length
                            }
                            onClick={() => {
                              const next = [...steps];
                              [next[index], next[index + direction]] = [
                                next[index + direction],
                                next[index],
                              ];
                              patch({ steps: next });
                              setActive(index + direction);
                            }}
                          >
                            {direction < 0 ? "↑" : "↓"}
                          </Button>
                        ))}
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-label={`Remove step ${index + 1}`}
                          className="h-6 w-6 p-0"
                          disabled={saving}
                          onClick={() => {
                            patch({
                              steps: steps.filter((_, i) => i !== index),
                            });
                            setActive(Math.max(0, index - 1));
                          }}
                        >
                          ×
                        </Button>
                      </div>
                    )}
                  </div>
                  {active === index && (
                    <div className="space-y-3 border-t border-border/50 px-4 pb-4 pt-3">
                      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <span>{index === 0 ? "Start after" : "Wait"}</span>
                        <QuietUnderlineInput
                          aria-label={`Step ${index + 1} delay in days`}
                          type="number"
                          min={0}
                          max={365}
                          value={step.delay_days}
                          onChange={(event) =>
                            patchStep(index, {
                              delay_days: Number(event.target.value),
                            })
                          }
                          className="w-12 py-1 text-center"
                          disabled={!editable || saving}
                        />
                        <span>days</span>
                        {step.kind === "email" && (
                          <div className="ml-auto">
                            <QuietSelect
                              label="Email delivery mode"
                              value={step.mode ?? "automatic"}
                              onChange={(value) =>
                                patchStep(index, {
                                  mode: value as "automatic" | "review",
                                })
                              }
                              options={[
                                {
                                  value: "automatic",
                                  label: "Send automatically",
                                },
                                {
                                  value: "review",
                                  label: "Review before sending",
                                },
                              ]}
                              disabled={!editable || saving}
                            />
                          </div>
                        )}
                      </div>
                      {step.kind === "email" ? (
                        <>
                          <QuietUnderlineInput
                            aria-label={`Step ${index + 1} subject`}
                            placeholder="Subject"
                            value={step.subject ?? ""}
                            onChange={(event) =>
                              patchStep(index, { subject: event.target.value })
                            }
                            disabled={!editable || saving}
                          />
                          <EmailContentEditor
                            key={index}
                            workspaceId={workspaceId}
                            value={step.body_html ?? ""}
                            onChange={(body_html) =>
                              patchStep(index, { body_html })
                            }
                            disabled={!editable || saving}
                            tools={
                              <EmailTemplatePicker
                                workspaceId={workspaceId}
                                disabled={!editable || saving}
                                onSelect={(template) =>
                                  patchStep(index, {
                                    subject: template.subject,
                                    body_html: template.body_html,
                                  })
                                }
                              />
                            }
                          />
                        </>
                      ) : (
                        <>
                          <QuietUnderlineInput
                            aria-label="Task title"
                            placeholder="Follow-up task"
                            value={step.task_name ?? ""}
                            onChange={(event) =>
                              patchStep(index, {
                                task_name: event.target.value,
                              })
                            }
                            disabled={!editable || saving}
                          />
                          <QuietSelect
                            label="Task team"
                            value={step.team_id ?? ""}
                            options={teams.teams.map((team) => ({
                              value: team.id,
                              label: team.name,
                            }))}
                            onChange={(team_id) =>
                              patchStep(index, { team_id })
                            }
                            disabled={!editable || saving}
                          />
                          <p className="text-xs text-muted-foreground">
                            Assigned to the sender. The sequence waits until
                            this task is completed.
                          </p>
                        </>
                      )}
                    </div>
                  )}
                </section>
              ))}
              {editable && steps.length < 20 && (
                <div className="flex justify-center gap-3 py-3">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => add("email")}
                    disabled={saving}
                  >
                    + Email step
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => add("task")}
                    disabled={saving}
                  >
                    + Task step
                  </Button>
                </div>
              )}
            </main>
            <aside className="min-w-0 space-y-5 text-sm">
              <details className="group/disclosure border-b border-border/50 pb-4">
                <summary className="flex cursor-pointer list-none items-center gap-2 rounded-md py-1 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden font-medium">
                  <ChevronRightIcon
                    aria-hidden="true"
                    className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-150 group-open/disclosure:rotate-90 motion-reduce:transition-none"
                  />
                  Enrollment
                </summary>
                <div className="mt-3 space-y-3">
                  <QuietSelect
                    label="Enrollment pipeline"
                    value={pipelineId}
                    onChange={(value) => {
                      setChosenPipeline(value);
                      patch({ entry_stage_id: "", entry_account_id: "" });
                    }}
                    options={[
                      { value: "manual", label: "Manual enrollment only" },
                      ...(pipelines.data ?? []).map((pipeline) => ({
                        value: pipeline.id,
                        label: pipeline.name,
                      })),
                    ]}
                    disabled={!editable || saving || pipelines.isLoading}
                  />
                  {pipelineId !== "manual" && (
                    <>
                      <QuietSelect
                        label="Enrollment stage"
                        value={form.entry_stage_id || "choose-stage"}
                        onChange={(entry_stage_id) => patch({ entry_stage_id })}
                        options={[
                          {
                            value: "choose-stage",
                            label: stages.length
                              ? "Select a stage"
                              : "No open stages",
                            disabled: true,
                          },
                          ...stages,
                        ]}
                        disabled={!editable || saving || !stages.length}
                      />
                      <p className="text-xs leading-5 text-muted-foreground">
                        Enroll the deal’s primary contact when it enters this
                        stage. Applies to future changes, once per recipient.
                      </p>
                    </>
                  )}
                  {form.entry_stage_id && (
                    <div className="space-y-2">
                      <p className="text-xs text-muted-foreground">
                        Sending mailbox
                      </p>
                      {accounts.isLoading ? (
                        <p className="text-xs text-muted-foreground">
                          Loading mailboxes…
                        </p>
                      ) : accounts.isError ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => void accounts.refetch()}
                        >
                          Couldn’t load mailboxes. Try again
                        </Button>
                      ) : mailboxes.length ? (
                        <QuietSelect
                          label="Sending mailbox"
                          placeholder="Select a mailbox"
                          value={form.entry_account_id || ""}
                          onChange={(entry_account_id) =>
                            patch({ entry_account_id })
                          }
                          options={mailboxes.map((account) => ({
                            value: account.id,
                            label: account.email_address,
                          }))}
                          disabled={!editable || saving}
                        />
                      ) : (
                        <div className="space-y-1 text-xs">
                          <p className="text-muted-foreground">
                            No sending mailbox connected.
                          </p>
                          {workspaceSlug && (
                            <a
                              className="underline underline-offset-4"
                              href={`/w/${workspaceSlug}/settings/crm-email`}
                              target="_blank"
                              rel="noopener noreferrer"
                            >
                              Connect a mailbox
                            </a>
                          )}
                        </div>
                      )}
                      {mailboxes.some(
                        (account) => account.id === form.entry_account_id,
                      ) && (
                        <p className="text-xs text-muted-foreground">
                          Saving authorizes this rule to send from your selected
                          mailbox.
                        </p>
                      )}
                    </div>
                  )}
                  <label className="block space-y-1 text-xs text-muted-foreground">
                    <QuickTooltip label="First emails to new recipients per rolling 24 hours, across this sequence’s mailboxes. Follow-ups take priority; extra recipients stay queued.">
                      <span
                        tabIndex={0}
                        className="cursor-help border-b border-dotted border-muted-foreground/40"
                      >
                        New recipients per day
                      </span>
                    </QuickTooltip>
                    <QuietUnderlineInput
                      aria-label="New recipients per day"
                      type="number"
                      min={1}
                      max={1000}
                      value={form.daily_new_recipients ?? 25}
                      onChange={(event) =>
                        patch({
                          daily_new_recipients: Number(event.target.value),
                        })
                      }
                      disabled={!editable || saving}
                    />
                  </label>
                  {form.entry_error && (
                    <p className="text-xs text-destructive">
                      {form.entry_error}
                    </p>
                  )}
                </div>
              </details>
              <details
                open
                className="group/disclosure border-b border-border/50 pb-4"
              >
                <summary className="flex cursor-pointer list-none items-center gap-2 rounded-md py-1 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden font-medium">
                  <ChevronRightIcon
                    aria-hidden="true"
                    className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform duration-150 group-open/disclosure:rotate-90 motion-reduce:transition-none"
                  />
                  Delivery
                </summary>
                <div className="mt-3 space-y-3">
                  <QuietSelect
                    label="Timezone"
                    value={form.timezone ?? "UTC"}
                    onChange={(timezone) => patch({ timezone })}
                    options={["UTC", ...Intl.supportedValuesOf("timeZone")].map(
                      (value) => ({ value, label: value.replaceAll("_", " ") }),
                    )}
                    disabled={!editable || saving}
                  />
                  <div className="grid grid-cols-2 gap-2">
                    <label className="text-xs text-muted-foreground">
                      From
                      <QuietSelect
                        label="Start hour"
                        value={String(form.start_hour)}
                        onChange={(value) =>
                          patch({ start_hour: Number(value) })
                        }
                        options={Array.from({ length: 24 }, (_, i) => ({
                          value: String(i),
                          label: `${i}:00`,
                        }))}
                        disabled={!editable || saving}
                      />
                    </label>
                    <label className="text-xs text-muted-foreground">
                      Until
                      <QuietSelect
                        label="End hour"
                        value={String(form.end_hour)}
                        onChange={(value) => patch({ end_hour: Number(value) })}
                        options={Array.from({ length: 24 }, (_, i) => ({
                          value: String(i + 1),
                          label: `${i + 1}:00`,
                        }))}
                        disabled={!editable || saving}
                      />
                    </label>
                  </div>
                  <label className="flex items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      checked={form.weekdays}
                      onChange={(event) =>
                        patch({ weekdays: event.target.checked })
                      }
                      disabled={!editable || saving}
                    />
                    Weekdays only
                  </label>
                  <label className="flex items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      checked={form.include_signature}
                      onChange={(event) =>
                        patch({ include_signature: event.target.checked })
                      }
                      disabled={!editable || saving}
                    />
                    Include sender’s signature
                  </label>
                </div>
              </details>
              <div className="space-y-2">
                <h3 className="font-medium">Stops automatically</h3>
                <p className="text-xs leading-5 text-muted-foreground">
                  Reply received · Unsubscribed · Undeliverable email · Deal won
                  or lost
                </p>
              </div>
              {editable && form.id && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-muted-foreground"
                  disabled={saving}
                  onClick={() => void save("archived")}
                >
                  Archive sequence
                </Button>
              )}
            </aside>
          </div>
        </div>
      )}
      {enroll && form.id && (
        <SequenceEnrollmentDialog
          workspaceId={workspaceId}
          sequenceId={form.id}
          onClose={() => setEnroll(false)}
        />
      )}
      <ConfirmDialog
        open={discard}
        onOpenChange={setDiscard}
        title="Discard sequence changes?"
        description="Your changes haven’t been saved."
        confirmLabel="Discard changes"
        cancelLabel="Keep editing"
        onConfirm={onClose}
      />
    </div>
  );
}
