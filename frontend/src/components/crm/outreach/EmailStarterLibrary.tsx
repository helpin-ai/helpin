import { useState } from "react";
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
  QuietUnderlineInput,
} from "@/components/design-system/quiet";
import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import { EmailBodyRenderer } from "@/components/support/EmailBodyRenderer";
import { emailStarters, buildStarterSteps } from "./emailStarters";
import type { EmailTemplate, EmailSequence } from "@/lib/crmOutreachTypes";
export function EmailStarterLibrary({
  mode,
  onClose,
  onTemplate,
  onSequence,
}: {
  mode: "templates" | "sequences";
  onClose: () => void;
  onTemplate: (template: Partial<EmailTemplate>) => void;
  onSequence: (sequence: Partial<EmailSequence>) => void;
}) {
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [id, setId] = useState(emailStarters[0].id);
  const [emailIndex, setEmailIndex] = useState(0);
  const [details, setDetails] = useState<
    Record<string, Record<string, string>>
  >({});
  const [discard, setDiscard] = useState(false);
  const rows = emailStarters.filter(
    (row) =>
      (category === "all" || row.category === category) &&
      `${row.name} ${row.category} ${row.when}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const starter =
    rows.find((row) => row.id === id) ?? rows[0] ?? emailStarters[0];
  const values = details[starter.id] ?? {};
  const picked = {
    ...starter,
    emails:
      mode === "templates" ? [starter.emails[emailIndex]] : starter.emails,
  };
  const usedText = picked.emails
    .map((e) => e.subject + e.paragraphs.join(" "))
    .join(" ");
  const fields = starter.fields.filter((field) =>
    usedText.includes(`[[${field.key}]]`),
  );
  let issue = "";
  let ready = true;
  let steps: ReturnType<typeof buildStarterSteps> = [];
  try {
    steps = buildStarterSteps(picked, values, true);
    buildStarterSteps(picked, values);
  } catch (error) {
    ready = false;
    issue = error instanceof Error ? error.message : "Complete the details";
  }
  const select = (value: string) => {
    setId(value);
    setEmailIndex(0);
  };
  const close = () =>
    Object.values(details).some((fields) =>
      Object.values(fields).some((v) => v.trim()),
    )
      ? setDiscard(true)
      : onClose();
  return (
    <>
      <Dialog open onOpenChange={(open) => !open && close()}>
        <DialogContent className="flex max-h-[92vh] flex-col overflow-hidden sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>
              {mode === "sequences" ? "Sequence starters" : "Email starters"}
            </DialogTitle>
            <DialogDescription>
              Choose a scenario, add your context, and make it yours.
            </DialogDescription>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-auto sm:grid sm:grid-cols-[220px_minmax(0,1fr)]">
            <aside className="space-y-3">
              <QuietSearchInput
                placeholder="Search scenarios"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
              <QuietSelect
                label="Starter category"
                value={category}
                onChange={setCategory}
                options={[
                  { value: "all", label: "All scenarios" },
                  ...["Prospecting", "Follow-up", "Customers"].map((value) => ({
                    value,
                    label: value,
                  })),
                ]}
              />
              <div className="sm:hidden">
                <QuietSelect
                  label="Starter scenario"
                  value={rows.length ? starter.id : ""}
                  onChange={select}
                  options={rows.map((row) => ({
                    value: row.id,
                    label: row.name,
                  }))}
                />
              </div>
              <div className="hidden space-y-1 sm:block">
                {rows.map((row) => (
                  <button
                    key={row.id}
                    onClick={() => select(row.id)}
                    aria-pressed={starter.id === row.id}
                    className={`w-full rounded-md px-3 py-2.5 text-left text-sm ${starter.id === row.id ? "bg-muted font-medium" : "hover:bg-muted/40"}`}
                  >
                    <span className="block">{row.name}</span>
                    <span className="mt-0.5 block text-xs font-normal text-muted-foreground">
                      {row.category}
                    </span>
                  </button>
                ))}
              </div>
              {!rows.length && (
                <p className="text-xs text-muted-foreground">
                  No matching scenarios.
                </p>
              )}
            </aside>
            <section className="min-w-0 space-y-4" hidden={!rows.length}>
              <div>
                <h3 className="hidden text-base font-semibold sm:block">
                  {starter.name}
                </h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {starter.when}
                </p>
              </div>
              {mode === "templates" && (
                <QuietSelect
                  label="Email in scenario"
                  value={String(emailIndex)}
                  onChange={(value) => setEmailIndex(Number(value))}
                  options={starter.emails.map((email, i) => ({
                    value: String(i),
                    label: email.label,
                  }))}
                />
              )}
              <div className="grid gap-x-5 gap-y-3 sm:grid-cols-2">
                {fields.map((field) => (
                  <label
                    key={field.key}
                    className="text-xs text-muted-foreground"
                  >
                    {field.label}
                    <QuietUnderlineInput
                      aria-label={field.label}
                      value={values[field.key] ?? ""}
                      maxLength={200}
                      placeholder={field.hint}
                      onChange={(event) =>
                        setDetails((current) => ({
                          ...current,
                          [starter.id]: {
                            ...current[starter.id],
                            [field.key]: event.target.value,
                          },
                        }))
                      }
                    />
                  </label>
                ))}
              </div>
              <div className="space-y-3 border-t border-border/50 pt-3">
                <p className="text-xs text-muted-foreground">
                  Preview · example recipient Alex
                </p>
                {steps.map((step, i) => {
                  const day =
                    1 +
                    steps
                      .slice(0, i + 1)
                      .reduce((total, item) => total + item.delay_days, 0);
                  return (
                    <details
                      key={`${starter.id}:${emailIndex}:${i}`}
                      open={i === 0}
                      className="border-b border-border/40 pb-3"
                    >
                      <summary className="cursor-pointer text-sm font-medium">
                        {mode === "sequences" && (
                          <span className="mr-2 text-xs font-normal text-muted-foreground">
                            Day {day}
                          </span>
                        )}
                        {picked.emails[i].label}
                      </summary>
                      <p className="my-3 text-sm font-medium">{step.subject}</p>
                      <EmailBodyRenderer
                        html={(step.body_html ?? "").replaceAll(
                          "{{first_name|there}}",
                          "Alex",
                        )}
                        collapsedByDefault={false}
                        constrainHeight={false}
                      />
                    </details>
                  );
                })}
              </div>
              <p className="text-xs text-muted-foreground">
                {mode === "sequences"
                  ? "Timing is a starting point; delivery windows still apply. Emails start in review mode and stop on reply or unsubscribe."
                  : "Recipient names personalize when you insert or send. Your mailbox signature is added separately."}
              </p>
            </section>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/50 pt-3">
            <span className="text-xs text-muted-foreground" aria-live="polite">
              {!rows.length
                ? "Try another search or category."
                : ready
                  ? "Opens as an editable draft."
                  : issue}
            </span>
            <Button
              disabled={!ready || !rows.length}
              onClick={() => {
                const content = buildStarterSteps(picked, values);
                if (mode === "templates")
                  onTemplate({
                    name: `${starter.name} · ${picked.emails[0].label}`,
                    subject: content[0].subject,
                    body_html: content[0].body_html,
                    shared: false,
                  });
                else
                  onSequence({
                    name: starter.name,
                    status: "draft",
                    version: 0,
                    steps: content,
                    weekdays: true,
                    include_signature: true,
                    entry_stage_id: "",
                    entry_account_id: "",
                  });
                onClose();
              }}
            >
              {mode === "templates"
                ? "Use email starter"
                : "Use sequence starter"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={discard}
        onOpenChange={setDiscard}
        title="Discard starter details?"
        description="Your setup details have not been saved."
        confirmLabel="Discard details"
        cancelLabel="Keep browsing"
        onConfirm={onClose}
      />
    </>
  );
}
