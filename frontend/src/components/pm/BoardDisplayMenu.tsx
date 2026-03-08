import { Settings2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Switch } from '@/components/ui/switch';
import {
  useBoardDisplayStore,
  DISPLAY_PROPERTY_LABELS,
  BOARD_PROPERTY_KEYS,
} from '@/stores/boardDisplayStore';

export function BoardDisplayMenu() {
  const { properties, showEmptyColumns, toggleProperty, toggleShowEmptyColumns } =
    useBoardDisplayStore();

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="h-7 w-7" title="Display settings">
          <Settings2 className="h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[240px] p-3" align="end">
        <div className="space-y-3">
          <div>
            <p className="mb-2 text-xs font-medium text-muted-foreground">Display properties</p>
            <div className="flex flex-wrap gap-1.5">
              {BOARD_PROPERTY_KEYS.map((key) => (
                <button
                  key={key}
                  onClick={() => toggleProperty(key)}
                  className={`rounded-md border px-2 py-1 text-xs font-medium transition-colors ${
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

          <div className="border-t border-border/70 pt-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium">Show empty columns</span>
              <Switch
                checked={showEmptyColumns}
                onCheckedChange={toggleShowEmptyColumns}
                className="scale-75"
              />
            </div>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
