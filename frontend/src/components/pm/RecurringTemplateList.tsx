import { TeamLabel } from '@/components/workspace/TeamLabel';
import { Copy01Icon, PauseIcon, PlayIcon, Forward01Icon, Delete01Icon, ZapIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import type { RecurringTemplateDetail } from '@/lib/pmTypes';
import { RecurringTemplateSummary } from '@/components/pm/RecurringTemplateSummary';

interface RecurringTemplateListProps {
  templates: RecurringTemplateDetail[];
  teamNames?: Map<string, string>;
  teamColors?: Map<string, string | null | undefined>;
  ownerNames?: Map<string, string>;
  onView?: (template: RecurringTemplateDetail) => void;
  onEdit?: (template: RecurringTemplateDetail) => void;
  onPause?: (template: RecurringTemplateDetail) => void;
  onResume?: (template: RecurringTemplateDetail) => void;
  onStop?: (template: RecurringTemplateDetail) => void;
  onSkipNext?: (template: RecurringTemplateDetail) => void;
  onGenerateNow?: (template: RecurringTemplateDetail) => void;
  onDuplicate?: (template: RecurringTemplateDetail) => void;
}

export function RecurringTemplateList({
  templates,
  teamNames,
  teamColors,
  ownerNames,
  onView,
  onEdit,
  onPause,
  onResume,
  onStop,
  onSkipNext,
  onGenerateNow,
  onDuplicate,
}: RecurringTemplateListProps) {
  if (templates.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border/60 px-4 py-8 text-center text-sm text-muted-foreground">
        No recurring templates match the current filters.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {templates.map((item) => {
        const teamName = item.template.team_id ? teamNames?.get(item.template.team_id) : undefined;
        const ownerName = item.template.owner_member_id ? ownerNames?.get(item.template.owner_member_id) : undefined;

        return (
          <div key={item.template.id} className="rounded-lg border border-border/60 bg-card p-4">
            <RecurringTemplateSummary
              title={item.template.title}
              status={item.template.status}
              ruleSummary={item.rule_summary}
              nextRunAt={item.template.next_run_at}
              generatedCount={item.template.generated_count}
              lastError={item.template.last_error}
              lastGeneratedTask={item.last_generated_task ?? null}
              compact
              actions={
                <>
                  {onView ? (
                    <Button type="button" variant="ghost" size="xs" onClick={() => onView(item)}>
                      History
                    </Button>
                  ) : null}
                  {onEdit ? (
                    <Button type="button" variant="ghost" size="xs" onClick={() => onEdit(item)}>
                      Edit
                    </Button>
                  ) : null}
                </>
              }
            />

            <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              {teamName ? <TeamLabel team={{name:teamName,color:teamColors?.get(item.template.team_id ?? '')}} /> : null}
              {ownerName ? <span>Owner: {ownerName}</span> : null}
              {item.seed.name ? <span>Task seed: {item.seed.name}</span> : null}
            </div>

            <div className="mt-3 flex flex-wrap gap-2">
              {item.template.status === 'active' ? (
                <Button type="button" variant="outline" size="xs" onClick={() => onPause?.(item)}>
                  <PauseIcon className="h-3 w-3" />
                  Pause
                </Button>
              ) : null}
              {item.template.status === 'paused' ? (
                <Button type="button" variant="outline" size="xs" onClick={() => onResume?.(item)}>
                  <PlayIcon className="h-3 w-3" />
                  Resume
                </Button>
              ) : null}
              <Button type="button" variant="outline" size="xs" onClick={() => onGenerateNow?.(item)}>
                <ZapIcon className="h-3 w-3" />
                Generate now
              </Button>
              <Button type="button" variant="outline" size="xs" onClick={() => onSkipNext?.(item)}>
                <Forward01Icon className="h-3 w-3" />
                Skip next
              </Button>
              <Button type="button" variant="outline" size="xs" onClick={() => onDuplicate?.(item)}>
                <Copy01Icon className="h-3 w-3" />
                Duplicate
              </Button>
              <Button type="button" variant="destructive" size="xs" onClick={() => onStop?.(item)}>
                <Delete01Icon className="h-3 w-3" />
                Delete
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
