import { Settings2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';

interface DisplayPropertiesPopoverProps {
  allProperties: { key: string; label: string }[];
  visible: string[];
  onChange: (next: string[]) => void;
}

export function DisplayPropertiesPopover({
  allProperties,
  visible,
  onChange,
}: DisplayPropertiesPopoverProps) {
  const toggle = (key: string) => {
    onChange(
      visible.includes(key)
        ? visible.filter((k) => k !== key)
        : [...visible, key]
    );
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-1.5 text-xs">
          <Settings2 className="h-3.5 w-3.5" />
          Display
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-3" align="end">
        <p className="mb-2.5 text-xs font-medium text-muted-foreground">Display properties</p>
        <div className="flex flex-wrap gap-1.5">
          {allProperties.map((prop) => {
            const active = visible.includes(prop.key);
            return (
              <button
                key={prop.key}
                type="button"
                onClick={() => toggle(prop.key)}
                className={`rounded-full border px-2.5 py-1 text-xs transition-colors cursor-pointer
                  ${active
                    ? 'border-border bg-accent font-medium text-foreground'
                    : 'border-border/60 text-muted-foreground hover:border-border hover:text-foreground'}
                `}
              >
                {prop.label}
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
