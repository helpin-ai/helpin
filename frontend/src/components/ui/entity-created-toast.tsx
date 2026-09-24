/* eslint-disable react-refresh/only-export-components -- Toast helper and its description share this UI module. */
import { toast } from 'sonner';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import type { IconComponent } from '@/lib/icons';
import {
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
  eyebrow?: string;
  openLabel?: string;
  tone?: CreatedEntityTone;
  icon?: IconComponent;
}

export function showEntityCreatedToast({
  entityLabel,
  title,
  identifier,
  onOpen,
  subtitle,
  eyebrow,
  openLabel = 'Open',
  tone = 'pm',
  icon,
}: CreatedEntityToastOptions) {
  const Icon = icon ?? (tone === 'pm' ? FolderKanbanIcon : Briefcase01Icon);

  return toast.success(eyebrow ?? `${entityLabel} created`, {
    description: <CreatedEntityToastDescription title={title} subtitle={subtitle} identifier={identifier} />,
    icon: <Icon className="size-4" aria-hidden="true" />,
    action: onOpen ? { label: openLabel, onClick: onOpen } : undefined,
    duration: 6000,
  });
}

function CreatedEntityToastDescription({
  title,
  subtitle,
  identifier,
}: Pick<CreatedEntityToastOptions, 'title' | 'subtitle' | 'identifier'>) {
  const identifierCopy = useCopyToClipboard();

  return (
    <div className="min-w-0 space-y-1">
      <span className="block break-words text-popover-foreground">{title}</span>
      {subtitle ? <span className="block">{subtitle}</span> : null}
      {identifier ? (
        <button
          type="button"
          className="inline-flex max-w-full items-center gap-1.5 rounded-md px-1 py-0.5 text-left text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
          aria-label={identifierCopy.copied ? `${identifier.label} copied` : `Copy ${identifier.label} ${identifier.value}`}
          onClick={() => identifierCopy.copy(identifier.value)}
        >
          <span>{identifier.label}</span>
          <span className="truncate font-mono text-foreground">{identifier.value}</span>
          {identifierCopy.copied
            ? <Tick01Icon className="size-3 shrink-0" aria-hidden="true" />
            : <Copy01Icon className="size-3 shrink-0" aria-hidden="true" />}
        </button>
      ) : null}
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
