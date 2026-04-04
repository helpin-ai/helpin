import { Settings02Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Switch } from '@/components/ui/switch';
import {
  useDealDisplayStore,
  DEAL_DISPLAY_PROPERTY_LABELS,
  DEAL_BOARD_PROPERTY_KEYS,
  DEAL_LIST_PROPERTY_KEYS,
} from '@/stores/dealDisplayStore';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

interface DealDisplayMenuProps {
  mode: 'board' | 'list';
}

export function DealDisplayMenu({ mode }: DealDisplayMenuProps) {
  const { properties, showEmptyStages, toggleProperty, toggleShowEmptyStages } =
    useDealDisplayStore();

  const keys = mode === 'board' ? DEAL_BOARD_PROPERTY_KEYS : DEAL_LIST_PROPERTY_KEYS;

  return (
    <Popover>
      <QuickTooltip label="Display settings">
        <PopoverTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7">
            <Settings02Icon className="h-4 w-4" />
          </Button>
        </PopoverTrigger>
      </QuickTooltip>
      <PopoverContent className="w-[240px] p-3" align="end">
        <div className="space-y-3">
          <div>
            <p className="mb-2 text-xs font-medium text-muted-foreground">Display properties</p>
            <div className="flex flex-wrap gap-1.5">
              {keys.map((key) => (
                <button
                  key={key}
                  onClick={() => toggleProperty(key)}
                  className={`rounded-md border px-2 py-1 text-xs font-medium transition-colors ${
                    properties[key]
                      ? 'border-primary/50 bg-primary/10 text-primary'
                      : 'border-border bg-background text-muted-foreground hover:bg-muted'
                  }`}
                >
                  {DEAL_DISPLAY_PROPERTY_LABELS[key]}
                </button>
              ))}
            </div>
          </div>

          {mode === 'board' && (
            <div className="border-t border-border/70 pt-3">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium">Show empty stages</span>
                <Switch
                  checked={showEmptyStages}
                  onCheckedChange={toggleShowEmptyStages}
                  className="scale-75"
                />
              </div>
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
