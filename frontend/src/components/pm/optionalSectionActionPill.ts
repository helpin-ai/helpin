import { cn } from '@/lib/utils';

export type OptionalSectionActionState = 'available' | 'open' | 'locked';
export type OptionalSectionActionVariant = 'pill' | 'borderless';

export function getOptionalSectionActionClass(
  state: OptionalSectionActionState,
  variant: OptionalSectionActionVariant = 'pill',
) {
  if (variant === 'borderless') {
    return cn(
      'inline-flex items-center gap-[7px] border-0 bg-transparent p-0 text-[13px] font-normal transition-colors',
      state === 'available' &&
        'cursor-pointer text-muted-foreground hover:text-foreground',
      state === 'open' &&
        'cursor-pointer font-medium text-foreground',
      state === 'locked' &&
        'cursor-default text-muted-foreground/70',
    );
  }

  return cn(
    'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors',
    state === 'available' &&
      'cursor-pointer border-primary/20 bg-primary/[0.025] text-primary/75 hover:border-primary/30 hover:bg-primary/[0.06] hover:text-primary',
    state === 'open' &&
      'cursor-pointer border-primary/25 bg-primary/[0.07] text-primary/85 hover:border-primary/35 hover:bg-primary/[0.1] hover:text-primary',
    state === 'locked' &&
      'cursor-default border-border/60 bg-muted/35 text-muted-foreground/80',
  );
}
