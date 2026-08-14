import { ColumnsThreeCogIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  LIST_PROPERTY_KEYS,
  type DisplayPropertyKey,
} from '@/stores/boardDisplayStore';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

interface ListDisplayMenuProps {
  /** Keys hidden at team-field-visibility level — these won't appear as toggleable. */
  disabledKeys?: Set<DisplayPropertyKey>;
}

export function ListDisplayMenu({ disabledKeys }: ListDisplayMenuProps) {
  const { properties, toggleProperty } = useBoardDisplayStore();

  const availableKeys = disabledKeys
    ? LIST_PROPERTY_KEYS.filter((k) => !disabledKeys.has(k))
    : LIST_PROPERTY_KEYS;

  return (
    <Popover>
      <QuickTooltip label="Display columns">
        <PopoverTrigger asChild>
          <Button variant="ghost" size="icon-sm">
            <ColumnsThreeCogIcon className="h-4 w-4" />
          </Button>
        </PopoverTrigger>
      </QuickTooltip>
      <PopoverContent className="w-[240px] p-3" align="end">
        <div>
          <p className="mb-2 text-ui font-medium text-muted-foreground">Display columns</p>
          <div className="flex flex-wrap gap-1.5">
            {availableKeys.map((key) => (
              <button
                key={key}
                onClick={() => toggleProperty(key)}
                className={`rounded-md border px-2 py-1 text-ui font-medium transition-colors ${
                  properties[key]
                    ? 'border-primary/50 bg-primary/10 text-primary'
                    : 'border-border bg-background text-muted-foreground hover:bg-muted'
                }`}
              >
                {DISPLAY_PROPERTY_LABELS[key]}
              </button>
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
