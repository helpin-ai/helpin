import type { ComponentProps } from 'react';
import { PopoverContent } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

/** Shared typography for PM field pickers. Filters use their own content. */
export function PMDropdownContent({ className, ...props }: ComponentProps<typeof PopoverContent>) {
  return (
    <PopoverContent
      className={cn(
        'text-sm [&_button]:text-sm [&_input]:text-sm [&_[data-slot=command-item]]:text-sm [&_[data-slot=command-empty]]:text-sm',
        className,
      )}
      {...props}
    />
  );
}
