import { useNavigate } from '@tanstack/react-router';
import {
  BookOpen01Icon,
  Briefcase01Icon,
  Cancel01Icon,
  FolderKanbanIcon,
  File01Icon,
  RecordIcon,
  UserIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CommandBarPageContext } from '@/lib/pmTypes';

interface PageContextBadgeProps {
  context: CommandBarPageContext;
  /** Called when the user explicitly clears the context. The badge then strips back to workspace fallback. */
  onClear?: () => void;
  /** Called when the user navigates to the entity. The palette typically closes itself. */
  onNavigate?: () => void;
}

const TYPE_LABEL: Record<CommandBarPageContext['entity_type'], string> = {
  task: 'Task',
  epic: 'Epic',
  document: 'Document',
  crm_contact: 'Contact',
  crm_deal: 'Deal',
  workspace: 'Workspace',
};

function iconFor(type: CommandBarPageContext['entity_type']) {
  switch (type) {
    case 'task':
      return RecordIcon;
    case 'epic':
      return BookOpen01Icon;
    case 'document':
      return File01Icon;
    case 'crm_contact':
      return UserIcon;
    case 'crm_deal':
      return Briefcase01Icon;
    case 'workspace':
    default:
      return FolderKanbanIcon;
  }
}

export function PageContextBadge({ context, onClear, onNavigate }: PageContextBadgeProps) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const Icon = iconFor(context.entity_type);
  const isFallback = context.entity_type === 'workspace';

  const handleClick = () => {
    if (!workspace?.slug) return;
    onNavigate?.();
    switch (context.entity_type) {
      case 'task':
        // Tasks are addressed by display id or task key; we have neither here, so just route to /pm/tasks?task=<id>
        window.location.assign(`/w/${workspace.slug}/pm/tasks?task=${context.entity_id}`);
        return;
      case 'epic':
        navigate({
          to: '/w/$slug/pm/epics/$epicId',
          params: { slug: workspace.slug, epicId: context.entity_id },
        });
        return;
      case 'document':
        navigate({
          to: '/w/$slug/docs/documents/$docId',
          params: { slug: workspace.slug, docId: context.entity_id },
        });
        return;
      case 'crm_contact':
        navigate({
          to: '/w/$slug/crm/contacts/$contactId',
          params: { slug: workspace.slug, contactId: context.entity_id },
        });
        return;
      case 'crm_deal':
        navigate({
          to: '/w/$slug/crm/deals/$dealId',
          params: { slug: workspace.slug, dealId: context.entity_id },
        });
        return;
      default:
        return;
    }
  };

  return (
    <div className="flex items-center gap-1.5 px-3 pt-2.5">
      <button
        type="button"
        onClick={isFallback ? undefined : handleClick}
        className={cn(
          'group inline-flex max-w-full items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] transition',
          isFallback
            ? 'border-border/70 bg-muted/40 text-muted-foreground cursor-default'
            : 'border-primary/20 bg-primary/5 text-foreground hover:bg-primary/10 cursor-pointer',
        )}
        title={isFallback ? 'Bound to current workspace' : `Open ${TYPE_LABEL[context.entity_type]}`}
      >
        <Icon className="h-3 w-3 shrink-0" />
        <Badge
          variant="outline"
          className="h-4 border-transparent bg-transparent px-1 py-0 text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
        >
          {TYPE_LABEL[context.entity_type] ?? context.entity_type}
        </Badge>
        <span className="truncate font-medium">{context.display_title || (isFallback ? workspace?.name : context.entity_id)}</span>
      </button>
      {!isFallback && onClear ? (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onClear();
          }}
          className="rounded-full p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
          title="Strip to workspace fallback"
        >
          <Cancel01Icon className="h-3 w-3" />
        </button>
      ) : null}
    </div>
  );
}
