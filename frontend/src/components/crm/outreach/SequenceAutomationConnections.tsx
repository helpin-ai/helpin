import { useEmailSequences } from "@/hooks/queries/useCRMOutreach";
export function SequenceAutomationConnections({
  workspaceId,
  slug,
}: {
  workspaceId: string;
  slug: string;
}) {
  const query = useEmailSequences(workspaceId);
  const rules = (query.data ?? []).filter(
    (s) => s.entry_stage_id && s.status !== "archived",
  );
  return (
    <details className="mb-5 border-b border-border/50 pb-4">
      <summary className="cursor-pointer text-sm font-medium">
        Email sequence enrollment{" "}
        <span className="ml-1 text-xs font-normal text-muted-foreground">
          {rules.length ? `${rules.length} configured` : ""}
        </span>
      </summary>
      <div className="mt-3 space-y-2 text-xs text-muted-foreground">
        <p>Enroll a deal’s primary contact when it enters a stage.</p>
        {rules.map((rule) => (
          <div
            key={rule.id}
            className="flex items-center justify-between gap-3"
          >
            <span>{rule.name}</span>
            <span className={rule.entry_error ? "text-destructive" : ""}>
              {rule.entry_error ||
                (rule.status === "active" ? "Active" : "Paused")}
            </span>
          </div>
        ))}
        <a
          className="inline-block text-foreground underline"
          href={`/w/${slug}/crm/emails?tab=sequences`}
        >
          Manage in CRM Emails →
        </a>
      </div>
    </details>
  );
}
