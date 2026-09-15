import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { QuietSearchInput } from "@/components/design-system/quiet";
import { useEmailTemplates } from "@/hooks/queries/useCRMOutreach";
import type { EmailTemplate } from "@/lib/crmOutreachTypes";
export function EmailTemplatePicker({
  workspaceId,
  onSelect,
  disabled,
}: {
  workspaceId: string;
  onSelect: (template: EmailTemplate) => void;
  disabled?: boolean;
}) {
  const query = useEmailTemplates(workspaceId);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const rows = (query.data ?? []).filter((row) =>
    `${row.name} ${row.subject}`.toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button size="sm" variant="ghost" disabled={disabled}>
          Templates
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-80 p-2">
        <QuietSearchInput
          placeholder="Search templates"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="mt-2 max-h-64 overflow-auto">
          {query.isLoading ? (
            <p className="p-3 text-xs text-muted-foreground">
              Loading templates…
            </p>
          ) : query.isError ? (
            <button
              onClick={() => void query.refetch()}
              className="p-3 text-sm"
            >
              Couldn’t load templates. Try again
            </button>
          ) : rows.length ? (
            rows.map((row) => (
              <button
                key={row.id}
                onClick={() => {
                  onSelect(row);
                  setOpen(false);
                }}
                className="w-full rounded-sm px-2 py-2 text-left hover:bg-muted/40"
              >
                <span className="block text-sm font-medium">{row.name}</span>
                <span className="block truncate text-xs text-muted-foreground">
                  {row.subject}
                </span>
              </button>
            ))
          ) : (
            <p className="p-3 text-xs text-muted-foreground">
              {search
                ? "No matching templates"
                : "Create your first template in CRM → Emails."}
            </p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
