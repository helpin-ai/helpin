import type { ComponentProps } from 'react';
import { PopoverContent } from '@/components/ui/popover';

/** Shared dropdown typography, including pickers rendered without Command. */
export function PMDropdownContent({ className, ...props }: ComponentProps<typeof PopoverContent>) {
  return (
    <PopoverContent
      data-dropdown-content=""
      className={className}
      {...props}
    />
  );
}
