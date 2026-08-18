import { cn } from '@/lib/utils';

interface HelpinLogoProps {
  className?: string;
  imageClassName?: string;
}

export function HelpinLogo({ className, imageClassName }: HelpinLogoProps) {
  return (
    <div
      aria-label="Helpin"
      data-helpin-brand-lockup
      className={cn('flex items-center justify-center gap-1 text-foreground', className)}
    >
      <img
        src="/brand/helpin-icon-ink.svg"
        alt=""
        aria-hidden="true"
        className={cn('h-9 w-9 shrink-0 dark:hidden', imageClassName)}
      />
      <img
        src="/brand/helpin-icon-white.svg"
        alt=""
        aria-hidden="true"
        className={cn('hidden h-9 w-9 shrink-0 dark:block', imageClassName)}
      />
      <span className="text-2xl font-semibold tracking-[-0.04em]">Helpin</span>
    </div>
  );
}
