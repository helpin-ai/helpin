import type { ComponentProps } from 'react';
import { PopoverContent } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

/** Shared typography for PM field pickers. Filters use their own content. */
export function PMDropdownContent({ className, ...props }: ComponentProps<typeof PopoverContent>) {
  return (
    <PopoverContent
      className={cn(
        'text-[length:var(--text-ui)] [&_button]:text-[length:var(--text-ui)] [&_input]:text-[length:var(--text-ui)] [&_[data-slot=command-item]]:text-[length:var(--text-ui)] [&_[data-slot=command-empty]]:text-[length:var(--text-ui)]',
        className,
      )}
      {...props}
    />
  );
}
