import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { DialogFooter } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { TeamFieldVisibility } from '@/lib/types';
import type { VisibilityFieldKey } from '@/lib/teamPresets';

type FieldVisibilityGroup = 'Classification' | 'Planning' | 'Other' | 'Panels';

export const FIELD_VISIBILITY_FIELDS: { key: VisibilityFieldKey; label: string; group: FieldVisibilityGroup }[] = [
  { key: 'priority', label: 'Priority', group: 'Classification' },
  { key: 'task_type', label: 'Type', group: 'Classification' },
  { key: 'severity', label: 'Severity', group: 'Classification' },
  { key: 'epic', label: 'Epic', group: 'Planning' },
  { key: 'sprint', label: 'Sprint', group: 'Planning' },
  { key: 'estimate', label: 'Estimate', group: 'Planning' },
  { key: 'labels', label: 'Labels', group: 'Other' },
  { key: 'due_date', label: 'Due Date', group: 'Other' },
  { key: 'blocked', label: 'Blocked', group: 'Other' },
  { key: 'delivery', label: 'Delivery', group: 'Panels' },
  { key: 'dev_history', label: 'Development History', group: 'Panels' },
];

const FIELD_VISIBILITY_HELP: Record<VisibilityFieldKey, string> = {
  priority: 'Shows the urgency level for a task so the team can quickly sort what matters most.',
  task_type: 'Shows whether the task is a feature, bug, or chore.',
  severity: 'Shows impact level, usually for bugs or operational issues. This starts off for new teams by default.',
  epic: 'Lets tasks roll up into larger initiatives.',
  sprint: 'Lets tasks be assigned to sprint cycles.',
  estimate: 'Shows effort sizing on tasks for planning and forecasting.',
  labels: 'Adds lightweight tags for categorization and filtering.',
  due_date: 'Shows target due dates directly on tasks.',
  blocked: 'Lets the team mark a task as blocked when it cannot move forward.',
  delivery: 'Shows delivery-related metadata such as repo and branch context.',
  dev_history: 'Shows linked pull requests, commits, and related development activity.',
};

export function FieldVisibilityForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamFieldVisibility | null;
  saving: boolean;
  onSave: (data: Partial<Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>>) => void;
}) {
  const [fields, setFields] = useState<Record<VisibilityFieldKey, boolean>>(() => {
    const defaults = {} as Record<VisibilityFieldKey, boolean>;
    for (const field of FIELD_VISIBILITY_FIELDS) {
      defaults[field.key] = initial ? initial[field.key] : field.key !== 'severity' && field.key !== 'blocked';
    }
    return defaults;
  });

  useEffect(() => {
    const next = {} as Record<VisibilityFieldKey, boolean>;
    for (const field of FIELD_VISIBILITY_FIELDS) {
      next[field.key] = initial ? initial[field.key] : field.key !== 'severity' && field.key !== 'blocked';
    }
    setFields(next);
  }, [initial, teamId]);

  const groups = [...new Set(FIELD_VISIBILITY_FIELDS.map((field) => field.group))];

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Configure which fields and panels appear on tasks for this team. State, Owner, Requester, and Team are always visible.
      </p>
      {groups.map((group) => (
        <div key={group} className="space-y-2">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{group}</p>
          {FIELD_VISIBILITY_FIELDS.filter((field) => field.group === group).map((field) => (
            <div key={field.key} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium">{field.label}</p>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="inline-flex h-4 w-4 items-center justify-center rounded-full border border-border/70 text-[10px] font-semibold text-muted-foreground transition-colors hover:border-border hover:text-foreground"
                      aria-label={`Help for ${field.label}`}
                    >
                      ?
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top" className="w-56 text-pretty leading-relaxed">
                    {FIELD_VISIBILITY_HELP[field.key]}
                  </TooltipContent>
                </Tooltip>
              </div>
              <Switch
                checked={fields[field.key]}
                onCheckedChange={(checked) => setFields((prev) => ({ ...prev, [field.key]: checked }))}
              />
            </div>
          ))}
        </div>
      ))}
      <DialogFooter>
        <Button disabled={saving} onClick={() => onSave(fields)}>
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}
