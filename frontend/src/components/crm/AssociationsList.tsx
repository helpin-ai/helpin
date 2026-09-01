import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  ArrowRight01Icon,
  Building03Icon,
  Camera01Icon,
  Delete01Icon,
  DollarCircleIcon,
  File01Icon,
  GitBranchIcon,
  Loading01Icon,
  Message01Icon,
  UserGroupIcon,
} from '@/lib/icons';
import { toast } from 'sonner';
import { Favicon } from '@/components/ui/favicon';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { QuietRelationshipDialogContent, QuietRelationshipResults, QuietSearchInput, quietRelatedItemTitleClassName, quietRelationshipResultRowClassName } from '@/components/design-system/quiet';
import {
  Dialog,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useCreateAssociation, useDeleteAssociation } from '@/hooks/queries/useCRM';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { supportService } from '@/lib/services/supportService';
import type { CRMAssociationEnriched, CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import type { ConversationStatus, SupportConversation } from '@/lib/pmTypes';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';
import { cn, truncateText } from '@/lib/utils';

interface AssociationsListProps {
  workspaceId: string;
  slug: string;
  associations: CRMAssociationEnriched[];
  currentObjectType: CRMObjectType;
  currentObjectId: string;
  onAssociationRemoved?: () => void;
  editable?: boolean;
  excludeTypes?: CRMObjectType[];
}

type SectionType = CRMObjectType;

const SECTION_PREVIEW_LIMIT = 3;

const sectionConfig: Record<SectionType, { title: string; icon: React.ElementType }> = {
  contact: { title: 'Contacts', icon: UserGroupIcon },
  company: { title: 'Companies', icon: Building03Icon },
  deal: { title: 'Deals', icon: DollarCircleIcon },
  meeting: { title: 'Meetings', icon: Camera01Icon },
  epic: { title: 'Epics', icon: File01Icon },
  task: { title: 'Tasks', icon: GitBranchIcon },
  support_conversation: { title: 'Support', icon: Message01Icon },
};

const crmAssociationTypes: CRMObjectType[] = ['contact', 'company', 'deal'];
const supportStatusDotClass: Record<ConversationStatus, string> = {
  open: 'bg-blue-500',
  waiting_on_customer: 'bg-purple-500',
  resolved: 'bg-green-500',
  spam: 'bg-red-500',
};

function getAssociationStatusDot(type: CRMObjectType, assoc: CRMAssociationEnriched) {
  if (type === 'support_conversation' && assoc.linked_object_status) {
    return supportStatusDotClass[assoc.linked_object_status as ConversationStatus] ?? 'bg-muted-foreground/40';
  }
  return null;
}

function humanizeStatusLabel(value: string) {
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function getAssociationStatusLabel(type: CRMObjectType, assoc: CRMAssociationEnriched) {
  if (!assoc.linked_object_status) return null;
  if (type === 'support_conversation') {
    return humanizeStatusLabel(assoc.linked_object_status);
  }
  return assoc.linked_object_status;
}

function AssociationsRailSection({
  title,
  count,
  expanded,
  onToggle,
  onAdd,
  children,
}: {
  title: string;
  count: number;
  expanded: boolean;
  onToggle: () => void;
  onAdd?: () => void;
  children: React.ReactNode;
}) {
  const canToggle = count > SECTION_PREVIEW_LIMIT;
  const hiddenCount = Math.max(count - SECTION_PREVIEW_LIMIT, 0);

  return (
    <>
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {title}
          {count > 0 && <span className="ml-1.5 font-normal">{count}</span>}
        </h3>
        {onAdd && (
          <button
            type="button"
            className="rounded-md p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            onClick={onAdd}
            aria-label={`Add ${title.toLowerCase()}`}
          >
            <span className="text-sm leading-none">+</span>
          </button>
        )}
      </div>

      {count > 0 && (
        <>
          <div className="mt-2 space-y-1">{children}</div>
          {canToggle && (
            <button
              type="button"
              className="mt-1 inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
              onClick={onToggle}
            >
              <ArrowRight01Icon className={cn('h-3 w-3 transition-transform', expanded && 'rotate-90')} />
              {expanded ? 'Show less' : `Show ${hiddenCount} more`}
            </button>
          )}
        </>
      )}
    </>
  );
}

export function AssociationsList({
  workspaceId,
  slug,
  associations,
  currentObjectType,
  currentObjectId,
  onAssociationRemoved,
  editable = true,
  excludeTypes = [],
}: AssociationsListProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const createAssociation = useCreateAssociation(workspaceId);
  const deleteAssociation = useDeleteAssociation(workspaceId);
  const [removeId, setRemoveId] = useState<string | null>(null);
  const [pickerSection, setPickerSection] = useState<SectionType | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [pmResults, setPMResults] = useState<SearchResult[]>([]);
  const [conversationResults, setConversationResults] = useState<SupportConversation[]>([]);
  const [expandedSections, setExpandedSections] = useState<Record<SectionType, boolean>>({
    contact: false,
    company: false,
    deal: false,
    meeting: false,
    epic: false,
    task: false,
    support_conversation: false,
  });

  const grouped = useMemo(() => {
    const groups: Record<SectionType, Array<CRMAssociationEnriched & { linkedType: CRMObjectType; linkedId: string }>> = {
      contact: [],
      company: [],
      deal: [],
      meeting: [],
      epic: [],
      task: [],
      support_conversation: [],
    };
    for (const assoc of associations) {
      const isFrom = assoc.from_object_type === currentObjectType && assoc.from_object_id === currentObjectId;
      const linkedType = isFrom ? assoc.to_object_type : assoc.from_object_type;
      const linkedId = isFrom ? assoc.to_object_id : assoc.from_object_id;
      groups[linkedType].push({ ...assoc, linkedType, linkedId });
    }
    return groups;
  }, [associations, currentObjectType, currentObjectId]);

  const handleNavigate = (type: CRMObjectType, id: string) => {
    const routes: Partial<Record<CRMObjectType, { to: string; params: Record<string, string> }>> = {
      contact: { to: '/w/$slug/crm/contacts/$contactId', params: { slug, contactId: id } },
      company: { to: '/w/$slug/crm/companies/$companyId', params: { slug, companyId: id } },
      meeting: { to: '/w/$slug/crm/meetings/$meetingId', params: { slug, meetingId: id } },
      epic: { to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: id } },
      support_conversation: { to: '/w/$slug/support/$conversationId', params: { slug, conversationId: id } },
    };
    if (type === 'task') {
      openTaskRoute(navigate as never, location as never, slug, id);
      return;
    }
    if (type === 'deal') {
      openDealRoute(navigate as never, location, slug, id);
      return;
    }
    const route = routes[type];
    if (route) navigate(route as never);
  };

  const handleRemove = async () => {
    if (!removeId) return;
    try {
      await deleteAssociation.mutateAsync(removeId);
      toast.success('Association removed');
      setRemoveId(null);
      onAssociationRemoved?.();
    } catch {
      toast.error('Failed to remove association');
    }
  };

  const closePicker = () => {
    setPickerSection(null);
    setQuery('');
    setCRMResults([]);
    setPMResults([]);
    setConversationResults([]);
    setSearching(false);
  };

  useEffect(() => {
    if (!pickerSection) return;

    const handle = window.setTimeout(async () => {
      if (pickerSection === 'support_conversation') {
        setSearching(true);
        const response = await supportService.listConversations(workspaceId);
        const items = response.data?.data ?? [];
        const normalized = query.trim().toLowerCase();
        setConversationResults(
          items.filter((c) => {
            if (!normalized) return true;
            return c.subject.toLowerCase().includes(normalized) || c.display_id.toString().includes(normalized);
          }),
        );
        setSearching(false);
        return;
      }

      if (query.trim().length < 2) {
        setCRMResults([]);
        setPMResults([]);
        return;
      }

      setSearching(true);
      if (pickerSection === 'contact' || pickerSection === 'company' || pickerSection === 'deal') {
        const response = await crmSearchService.search(workspaceId, query.trim());
        setCRMResults((response.data ?? []).filter((r) => r.type === pickerSection));
      } else if (pickerSection === 'epic' || pickerSection === 'task') {
        const response = await searchService.search(workspaceId, query.trim());
        const items = pickerSection === 'epic' ? (response.data?.epics ?? []) : (response.data?.tasks ?? []);
        setPMResults(items);
      }
      setSearching(false);
    }, 250);

    return () => window.clearTimeout(handle);
  }, [pickerSection, query, workspaceId]);

  const handleAdd = async (toType: CRMObjectType, toId: string) => {
    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: currentObjectType,
      from_object_id: currentObjectId,
      to_object_type: toType,
      to_object_id: toId,
    });
    closePicker();
    onAssociationRemoved?.();
  };

  const sectionOrder: SectionType[] = ['contact', 'company', 'deal', 'meeting', 'epic', 'task', 'support_conversation'];
  const excludedTypeSet = new Set(excludeTypes);
  const visibleSections = sectionOrder.filter(
    (type) => type !== currentObjectType && !excludedTypeSet.has(type),
  );
  const pickerConfig = pickerSection ? sectionConfig[pickerSection] : null;
  const pickerPlaceholder =
    pickerSection === 'support_conversation'
      ? 'Filter conversations by subject or ID'
      : pickerSection === 'epic' || pickerSection === 'task'
        ? `Search ${pickerSection}s by title or ID`
        : `Search ${pickerSection ? `${pickerSection}s` : ''}`;
  const pickerIcon = pickerConfig?.icon ?? File01Icon;
  const PickerIcon = pickerIcon;

  return (
    <div className="py-4">
      {visibleSections.map((type, index) => {
        const items = grouped[type];
        const config = sectionConfig[type];
		const visibleItems = expandedSections[type] ? items : items.slice(0, SECTION_PREVIEW_LIMIT);
		const dealCompanyPair = (currentObjectType === 'company' && type === 'deal') || (currentObjectType === 'deal' && type === 'company');

        return (
          <div key={type}>
            {index > 0 && <div className="my-4 h-px bg-border/60" />}
            <AssociationsRailSection
              title={config.title}
              count={items.length}
              expanded={expandedSections[type]}
              onToggle={() => setExpandedSections((current) => ({ ...current, [type]: !current[type] }))}
			onAdd={!editable || type === 'meeting' || dealCompanyPair ? undefined : () => setPickerSection(type)}
            >
              {visibleItems.map((assoc) => {
                const Icon = config.icon;
                const isCRMRecord = crmAssociationTypes.includes(type);
                const rowKey = assoc.id || `inferred-${assoc.linkedType}-${assoc.linkedId}-${assoc.context_label ?? 'association'}`;

                return (
                  <div
                    key={rowKey}
                    className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      onClick={() => handleNavigate(assoc.linkedType, assoc.linkedId)}
                    >
                      {type === 'task' || type === 'support_conversation' ? (
                        (() => {
                          const statusLabel = getAssociationStatusLabel(type, assoc);
                          const dot = (
                            <span
                              className={cn('h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30', getAssociationStatusDot(type, assoc))}
                              style={type === 'task' && assoc.linked_object_status_color ? { backgroundColor: assoc.linked_object_status_color } : undefined}
                              aria-label={statusLabel ?? undefined}
                            />
                          );
                          return statusLabel ? (
                            <QuickTooltip label={statusLabel}>
                              {dot}
                            </QuickTooltip>
                          ) : dot;
                        })()
                      ) : (
                        <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                      )}
                      <span className={cn('truncate', quietRelatedItemTitleClassName)}>
                        {assoc.linked_object_name || assoc.linkedType}
                      </span>
                      {(assoc.context_label || assoc.linked_object_display_id) && (
                        <span className="ml-auto flex shrink-0 items-center gap-2">
                          {assoc.context_label && (
                            <span className="text-[10px] text-muted-foreground">{assoc.context_label}</span>
                          )}
                          {assoc.linked_object_display_id && (
                            <span className={cn(
                              'text-[10px] text-muted-foreground',
                              isCRMRecord && 'opacity-0 transition-opacity delay-0 group-hover:opacity-100 group-hover:delay-200',
                            )}>
                              {assoc.linked_object_display_id}
                            </span>
                          )}
                        </span>
                      )}
                    </button>
					{editable && assoc.id && assoc.association_label !== 'deal_customer' ? (
                      <button
                        type="button"
                        className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                        onClick={() => setRemoveId(assoc.id)}
                        aria-label="Remove association"
                      >
                        <Delete01Icon className="h-3 w-3" />
                      </button>
                    ) : null}
                  </div>
                );
              })}
            </AssociationsRailSection>
          </div>
        );
      })}

      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) closePicker(); }}>
        <QuietRelationshipDialogContent>
          <DialogHeader>
            <DialogTitle className="text-sm">Link {pickerConfig?.title?.replace(/s$/, '') ?? ''}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <QuietSearchInput
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={pickerPlaceholder}
              autoFocus
            />
            <QuietRelationshipResults className="-mx-1 max-h-80 overflow-y-auto px-1">
              {searching && (
                <div className="flex items-center justify-center gap-2 py-4 text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}

              {!searching && pickerSection === 'support_conversation' && conversationResults.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={cn(quietRelationshipResultRowClassName, 'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent')}
                  onClick={() => handleAdd('support_conversation', c.id)}
                >
                  <span className="min-w-0 flex-1 truncate">{truncateText(c.subject, 60)}</span>
                  <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
                    C-{c.display_id}
                  </span>
                </button>
              ))}

              {!searching && (pickerSection === 'contact' || pickerSection === 'company' || pickerSection === 'deal') && crmResults.map((r) => (
                <button
                  key={`${r.type}-${r.id}`}
                  type="button"
                  className={cn(quietRelationshipResultRowClassName, 'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent')}
                  onClick={() => handleAdd(r.type as CRMObjectType, r.id)}
                >
                  {r.type === 'company' ? (
                    <Favicon
                      src={'logo_url' in r.object ? r.object.logo_url : undefined}
                      url={'domain' in r.object ? r.object.domain : undefined}
                      name={r.name}
                      size={16}
                      className="h-3.5 w-3.5 shrink-0 rounded-sm border-none bg-transparent"
                      fallbackClassName="text-[7px]"
                    />
                  ) : (
                    <PickerIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  )}
                  <span className="min-w-0 flex-1 truncate">{truncateText(r.name, 60)}</span>
                </button>
              ))}

              {!searching && (pickerSection === 'epic' || pickerSection === 'task') && pmResults.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className={cn(quietRelationshipResultRowClassName, 'flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent')}
                  onClick={() => handleAdd(pickerSection, r.id)}
                >
                  <PickerIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate">{truncateText(r.name, 60)}</span>
                  {r.display_id && (
                    <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
                      {r.display_id}
                    </span>
                  )}
                </button>
              ))}

              {!searching && pickerSection === 'support_conversation' && conversationResults.length === 0 && (
                <p className="py-4 text-center text-sm text-muted-foreground">No results found</p>
              )}
              {!searching && pickerSection !== 'support_conversation' && query.trim().length >= 2 &&
                (((pickerSection === 'contact' || pickerSection === 'company' || pickerSection === 'deal') && crmResults.length === 0) ||
                  ((pickerSection === 'epic' || pickerSection === 'task') && pmResults.length === 0)) && (
                  <p className="py-4 text-center text-sm text-muted-foreground">No results found</p>
                )}
              {!searching && pickerSection !== 'support_conversation' && query.trim().length < 2 && (
                <p className="py-4 text-center text-sm text-muted-foreground">Type at least 2 characters to search</p>
              )}
            </QuietRelationshipResults>
          </div>
        </QuietRelationshipDialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!removeId}
        onOpenChange={(open) => !open && setRemoveId(null)}
        title="Remove association"
        description="Are you sure you want to remove this association?"
        confirmLabel="Remove"
        variant="destructive"
        onConfirm={handleRemove}
      />
    </div>
  );
}
