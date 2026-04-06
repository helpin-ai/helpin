import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { cn } from '@/lib/utils';
import type { IconComponent } from '@/lib/icons';
import {
  ArrowUpRight01Icon,
  Briefcase01Icon,
  Building03Icon,
  Copy01Icon,
  FolderKanbanIcon,
  Layers01Icon,
  Tick01Icon,
  UserIcon,
} from '@/lib/icons';

type CreatedEntityTone = 'pm' | 'crm';

interface CreatedEntityIdentifier {
  label: string;
  value: string;
}

interface CreatedEntityToastOptions {
  entityLabel: string;
  title: string;
  identifier?: CreatedEntityIdentifier;
  onOpen?: () => void;
  subtitle?: string;
  tone?: CreatedEntityTone;
  icon?: IconComponent;
}

interface CreatedEntityToastCardProps extends CreatedEntityToastOptions {
  toastId: string | number;
}

const toneIconClasses: Record<CreatedEntityTone, string> = {
  pm: 'bg-primary/12 text-primary',
  crm: 'bg-secondary text-secondary-foreground',
};

export function showEntityCreatedToast(options: CreatedEntityToastOptions) {
  return toast.custom((toastId) => (
    <CreatedEntityToastCard
      {...options}
      toastId={toastId}
    />
  ), {
    duration: 12000,
  });
}

function CreatedEntityToastCard({
  toastId,
  entityLabel,
  title,
  identifier,
  onOpen,
  subtitle,
  tone = 'pm',
  icon,
}: CreatedEntityToastCardProps) {
  const Icon = icon ?? (tone === 'pm' ? FolderKanbanIcon : Briefcase01Icon);
  const identifierCopy = useCopyToClipboard();

  const handleOpen = () => {
    onOpen?.();
    toast.dismiss(toastId);
  };

  const handleCopyIdentifier = () => {
    if (!identifier) return;
    identifierCopy.copy(identifier.value);
  };

  return (
    <div
      className={cn(
        'relative w-[min(420px,calc(100vw-2rem))] overflow-hidden rounded-[1.05rem] border border-border bg-popover text-popover-foreground shadow-[0_20px_55px_-32px_hsl(var(--foreground)/0.35)]',
      )}
    >
      <div className="pointer-events-none absolute inset-x-0 top-0 h-14 bg-[radial-gradient(circle_at_top_left,hsl(var(--primary)/0.12),transparent_62%)]" />
      <div className="pointer-events-none absolute inset-y-0 left-0 w-1 bg-primary/30" />
      <div className="relative p-4">
        <div className="flex items-start gap-3">
          <div className={cn('mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-border/70', toneIconClasses[tone])}>
            <Icon className="h-5 w-5" />
          </div>

          <div className="min-w-0 flex-1">
            <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              {entityLabel} created
            </p>
            <button
              type="button"
              className={cn(
                'mt-1 block max-w-full truncate text-left text-sm font-semibold leading-5 transition-colors',
                onOpen ? 'cursor-pointer hover:text-primary' : 'cursor-default',
              )}
              disabled={!onOpen}
              onClick={handleOpen}
            >
              {title}
            </button>
            {subtitle ? (
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {subtitle}
              </p>
            ) : null}
          </div>
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          {identifier ? (
            <span className="inline-flex items-center rounded-full border border-border bg-muted/70 px-2.5 py-1 font-mono text-[11px] text-foreground shadow-sm">
              <span className="mr-1.5 font-medium uppercase tracking-[0.12em] text-muted-foreground">
                {identifier.label}
              </span>
              {identifier.value}
            </span>
          ) : null}
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          {identifier ? (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="h-8 rounded-full px-3 text-xs shadow-sm"
              onClick={handleCopyIdentifier}
            >
              {identifierCopy.copied ? <Tick01Icon className="mr-1.5 h-3.5 w-3.5" /> : <Copy01Icon className="mr-1.5 h-3.5 w-3.5" />}
              {identifierCopy.copied ? `${identifier.label} copied` : `Copy ${identifier.label}`}
            </Button>
          ) : null}
          {onOpen ? (
            <Button
              type="button"
              size="sm"
              className="h-8 rounded-full px-3 text-xs"
              onClick={handleOpen}
            >
              <ArrowUpRight01Icon className="mr-1.5 h-3.5 w-3.5" />
              Open
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export const entityCreatedToastIcons = {
  task: Layers01Icon,
  epic: FolderKanbanIcon,
  contact: UserIcon,
  company: Building03Icon,
  deal: Briefcase01Icon,
} as const;
