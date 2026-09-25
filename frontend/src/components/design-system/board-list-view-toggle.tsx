import { Button } from '@/components/ui/button';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { LayoutTwoColumnIcon, LayoutTable01Icon } from '@/lib/icons';
/** The task board/list switcher, shared by CRM. */
export function BoardListViewToggle({
  value,
  onChange,
}: {
  value: 'board' | 'list';
  onChange: (value: 'board' | 'list') => void;
}) {
  return (
    <span className="inline-flex h-7 items-center gap-0.5 rounded-md border border-border/70 bg-muted/30 p-0.5">
      {(
        [
          { value: 'board', label: 'Board view', icon: LayoutTwoColumnIcon },
          { value: 'list', label: 'List view', icon: LayoutTable01Icon },
        ] as const
      ).map((option) => (
        <QuickTooltip key={option.value} label={option.label}>
          <Button
            variant="ghost"
            size="icon"
            aria-label={option.label}
            aria-pressed={value === option.value}
            className={`h-6 w-6 rounded-sm ${value === option.value ? 'bg-background text-foreground shadow-sm hover:bg-background' : 'text-muted-foreground/60 hover:text-foreground'}`}
            onClick={() => onChange(option.value)}
          >
            <option.icon className="h-3.5 w-3.5" />
          </Button>
        </QuickTooltip>
      ))}
    </span>
  );
}
