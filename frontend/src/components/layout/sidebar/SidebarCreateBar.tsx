import { ArrowDown01Icon, PlusSignIcon, type IconComponent } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

type SidebarCreateOption = {
  key: string;
  label: string;
  icon?: IconComponent;
  onSelect: () => void;
};

type SidebarCreateBarProps = {
  primaryLabel: string;
  onPrimaryClick: () => void;
  options: SidebarCreateOption[];
  className?: string;
  moreLabel?: string;
};

export function SidebarCreateBar({
  primaryLabel,
  onPrimaryClick,
  options,
  className,
  moreLabel = 'More create options',
}: SidebarCreateBarProps) {
  return (
    <div className={cn('mb-2 flex w-full px-1', className)}>
      <Button
        size="sm"
        className="h-7 flex-1 gap-1.5 rounded-r-none text-xs"
        onClick={onPrimaryClick}
      >
        <PlusSignIcon className="h-3 w-3" />
        {primaryLabel}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            size="sm"
            className="h-7 rounded-l-none border-l border-primary-foreground/20 px-1.5"
            aria-label={moreLabel}
          >
            <ArrowDown01Icon className="h-3 w-3" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {options.map((option) => (
            <DropdownMenuItem key={option.key} onClick={option.onSelect}>
              {option.icon && <option.icon className="h-4 w-4" />}
              {option.label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
