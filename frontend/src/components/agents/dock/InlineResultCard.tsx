import { cn } from '@/lib/utils';

interface InlineResultCardProps {
  children: React.ReactNode;
  className?: string;
}

/**
 * Soft inline card for an agent run's textual result. No border — reads as part
 * of the answer instead of as a foreign card.
 */
export function InlineResultCard({ children, className }: InlineResultCardProps) {
  return (
    <div className={cn('rounded-lg bg-secondary/60 px-3 py-2 text-xs leading-5 text-foreground', className)}>
      {children}
    </div>
  );
}
