import type { LucideIcon } from 'lucide-react';
import { ChevronDown, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

type SidebarCreateOption = {
  key: string;
  label: string;
  icon?: LucideIcon;
  onSelect: () => void;
};

type SidebarCreateBarProps = {
  primaryLabel: string;
  onPrimaryClick: () => void;
  options: SidebarCreateOption[];
};

export function SidebarCreateBar({
  primaryLabel,
  onPrimaryClick,
  options,
}: SidebarCreateBarProps) {
  return (
    <div className="mb-2 flex w-full">
      <Button
        size="sm"
        className="h-7 flex-1 gap-1.5 rounded-r-none text-xs"
        onClick={onPrimaryClick}
      >
        <Plus className="h-3 w-3" />
        {primaryLabel}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="sm" className="h-7 rounded-l-none border-l border-primary-foreground/20 px-1.5">
            <ChevronDown className="h-3 w-3" />
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
