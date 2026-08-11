import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  ArrowRight01Icon,
  Building03Icon,
  Delete01Icon,
  DollarCircleIcon,
  File01Icon,
  Loading01Icon,
  Search01Icon,
  UserGroupIcon,
} from '@/lib/icons';

import {
  useCreateDocAssociation,
  useCreatePMAssociation,
  useDeleteDocAssociation,
  useDeletePMAssociation,
  useEpicAssociations,
  useTaskAssociations,
} from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { supportService } from '@/lib/services/supportService';
import { cn, truncateText } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSearchResult, CRMObjectType } from '@/lib/crmTypes';
import type {
  AssociationObjectSummary,
  ConversationStatus,
  GroupedAssociations,
  SupportConversation,
} from '@/lib/pmTypes';
import { Input } from '@/components/ui/input';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { DocumentPreviewDialog } from '@/components/docs/DocumentPreviewDialog';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

type AssociationsObjectType = 'task' | 'epic';
type SectionKey = 'support' | 'crm' | 'docs';
const SECTION_PREVIEW_LIMIT = 3;

const crmIconMap = {
  contact: UserGroupIcon,
  company: Building03Icon,
  deal: DollarCircleIcon,
} as const;

const supportStatusDotClass: Record<ConversationStatus, string> = {
  open: 'bg-blue-500',
  waiting_on_customer: 'bg-purple-500',
  resolved: 'bg-green-500',
  spam: 'bg-red-500',
};

function humanizeStatusLabel(value: string) {
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function AssociationsRailSection({
  title,
  count,
  emptyState,
  expanded,
  onToggle,
  onAdd,
  children,
}: {
  title: string;
  count: number;
  emptyState: string;
  expanded: boolean;
  onToggle: () => void;
  onAdd: () => void;
  children: React.ReactNode;
}) {
  const canToggle = count > SECTION_PREVIEW_LIMIT;
  const hiddenCount = Math.max(count - SECTION_PREVIEW_LIMIT, 0);

  return (
    <>
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">
          {title}
          {count > 0 && <span className="ml-1.5 font-normal">{count}</span>}
        </h3>
        <button
          type="button"
          className="rounded-md p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          onClick={onAdd}
          aria-label={`Add ${title.toLowerCase()}`}
        >
          <span className="text-sm leading-none">+</span>
        </button>
      </div>

      {count === 0 ? (
        <p className="mt-2 py-2 text-[11px] italic text-muted-foreground">{emptyState}</p>
      ) : (
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

interface AssociationsPanelProps {
  workspaceId: string;
  objectType: AssociationsObjectType;
  objectId: string;
  className?: string;
  includeTaskRelationships?: boolean;
  excludeDocs?: boolean;
}

export function AssociationsPanel({
  workspaceId,
  objectType,
  objectId,
  className,
  excludeDocs = false,
}: AssociationsPanelProps) {
  const navigate = useNavigate();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const [previewDocId, setPreviewDocId] = useState<string | null>(null);

  const handleNavigate = (item: AssociationObjectSummary) => {
    const type = item.object_type;
    const id = item.object_id;
    if (type === 'support_conversation') {
      navigate({ to: '/w/$slug/support/$conversationId', params: { slug, conversationId: id } } as any);
    } else if (type === 'contact') {
      navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug, contactId: id } } as any);
    } else if (type === 'company') {
      navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug, companyId: id } } as any);
    } else if (type === 'deal') {
      navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug, dealId: id } } as any);
    } else if (type === 'document') {
      setPreviewDocId(id);
    }
  };

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [docResults, setDocResults] = useState<SearchResult[]>([]);
  const [conversationResults, setConversationResults] = useState<SupportConversation[]>([]);
  const [expandedSections, setExpandedSections] = useState<Record<SectionKey, boolean>>({
    support: false,
    crm: false,
    docs: false,
  });

  const associationsQuery =
    objectType === 'task'
      ? useTaskAssociations(workspaceId, objectId)
      : useEpicAssociations(workspaceId, objectId);
  const data = associationsQuery.data as GroupedAssociations | undefined;

  const createAssociation = useCreatePMAssociation(workspaceId);
  const deleteAssociation = useDeletePMAssociation(workspaceId);
  const createDocAssociation = useCreateDocAssociation(workspaceId, objectType, objectId);
  const deleteDocAssociation = useDeleteDocAssociation(workspaceId, objectType, objectId);

  useEffect(() => {
    if (!pickerSection) {
      setQuery('');
      setCRMResults([]);
      setDocResults([]);
      setConversationResults([]);
      setSearching(false);
      return;
    }

    const handle = window.setTimeout(async () => {
      if (pickerSection === 'support') {
        setSearching(true);
        const response = await supportService.listConversations(workspaceId);
        const items = response.data?.data ?? [];
        const normalized = query.trim().toLowerCase();
        setConversationResults(
          items.filter((conversation) => {
            if (!normalized) return true;
            return (
              conversation.subject.toLowerCase().includes(normalized) ||
              conversation.display_id.toString().includes(normalized)
            );
          })
        );
        setSearching(false);
        return;
      }

      if (query.trim().length < 2) {
        setCRMResults([]);
        setDocResults([]);
        return;
      }

      setSearching(true);
      if (pickerSection === 'crm') {
        const response = await crmSearchService.search(workspaceId, query.trim());
        setCRMResults(response.data ?? []);
      } else if (pickerSection === 'docs') {
        const response = await searchService.search(workspaceId, query.trim());
        setDocResults(response.data?.documents ?? []);
      }
      setSearching(false);
    }, 250);

    return () => window.clearTimeout(handle);
  }, [pickerSection, query, workspaceId]);

  const handleAddCRM = async (toObjectType: CRMObjectType, toObjectId: string) => {
    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: objectType,
      from_object_id: objectId,
      to_object_type: toObjectType,
      to_object_id: toObjectId,
    });
    setPickerSection(null);
  };

  const handleAddSupport = async (conversationId: string) => {
    await createAssociation.mutateAsync({
      workspace_id: workspaceId,
      from_object_type: objectType,
      from_object_id: objectId,
      to_object_type: 'support_conversation',
      to_object_id: conversationId,
    });
    setPickerSection(null);
  };

  const handleAddDoc = async (documentId: string) => {
    await createDocAssociation.mutateAsync({
      documentId,
      payload: {
        linked_object_type: objectType,
        linked_object_id: objectId,
        link_context: 'attached',
      },
    });
    setPickerSection(null);
  };

  if (associationsQuery.isLoading) {
    return (
      <div className={cn('flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground', className)}>
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading...
      </div>
    );
  }

  const supportConversations = data?.support_conversations ?? [];
  const crmRecords = data?.crm_records ?? [];
  const docs = data?.docs ?? [];
  const visibleSupportConversations = expandedSections.support
    ? supportConversations
    : supportConversations.slice(0, SECTION_PREVIEW_LIMIT);
  const visibleCRMRecords = expandedSections.crm
    ? crmRecords
    : crmRecords.slice(0, SECTION_PREVIEW_LIMIT);
  const visibleDocs = expandedSections.docs
    ? docs
    : docs.slice(0, SECTION_PREVIEW_LIMIT);

  const pickerTitle =
    pickerSection === 'support' ? 'Support Conversation' :
    pickerSection === 'crm' ? 'CRM Record' :
    'Document';

  const pickerPlaceholder =
    pickerSection === 'support' ? 'Filter conversations by subject or ID' :
    pickerSection === 'crm' ? 'Search contacts, companies, or deals' :
    'Search documents';

  return (
    <div className={cn('px-3 py-4', className)}>
      <AssociationsRailSection
        title="Support"
        count={supportConversations.length}
        emptyState="No linked support conversations"
        expanded={expandedSections.support}
        onToggle={() => setExpandedSections((current) => ({ ...current, support: !current.support }))}
        onAdd={() => setPickerSection('support')}
      >
        {visibleSupportConversations.map((item) => (
          <div
            key={`${item.object_type}-${item.object_id}`}
            className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-muted/40"
          >
            <button
              type="button"
              className="flex min-w-0 flex-1 items-center gap-2 text-left"
              onClick={() => handleNavigate(item)}
            >
              {item.status && (
                <QuickTooltip label={humanizeStatusLabel(item.status)}>
                  <span
                    className={cn(
                      'h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/40',
                      supportStatusDotClass[item.status as ConversationStatus] ?? 'bg-muted-foreground/40',
                    )}
                    aria-label={humanizeStatusLabel(item.status)}
                  />
                </QuickTooltip>
              )}
              <span className="truncate font-medium">{item.title}</span>
              {item.display_id && (
                <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                  {item.display_id}
                </span>
              )}
            </button>
            {item.association_id ? (
              <button
                type="button"
                className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                onClick={() => deleteAssociation.mutate(item.association_id!)}
                aria-label="Remove support association"
              >
                <Delete01Icon className="h-3 w-3" />
              </button>
            ) : null}
          </div>
        ))}
      </AssociationsRailSection>

      <div className="my-4 h-px bg-border/60" />

      <AssociationsRailSection
        title="CRM"
        count={crmRecords.length}
        emptyState="No linked CRM records"
        expanded={expandedSections.crm}
        onToggle={() => setExpandedSections((current) => ({ ...current, crm: !current.crm }))}
        onAdd={() => setPickerSection('crm')}
      >
        {visibleCRMRecords.map((item) => {
          const CRMIcon = crmIconMap[item.object_type as keyof typeof crmIconMap] ?? Building03Icon;

          return (
            <div
              key={`${item.object_type}-${item.object_id}`}
              className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-muted/40"
            >
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-2 text-left"
                onClick={() => handleNavigate(item)}
              >
                <CRMIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="truncate font-medium">{item.title}</span>
                {(item.context_label || item.display_id) && (
                  <span className="ml-auto flex shrink-0 items-center gap-2">
                    {item.context_label && (
                      <span className="text-[10px] text-muted-foreground">{item.context_label}</span>
                    )}
                    {item.display_id && (
                      <span className="text-[10px] text-muted-foreground opacity-0 transition-opacity delay-0 group-hover:opacity-100 group-hover:delay-200">
                        {item.display_id}
                      </span>
                    )}
                  </span>
                )}
              </button>
              {item.association_id ? (
                <button
                  type="button"
                  className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                  onClick={() => deleteAssociation.mutate(item.association_id!)}
                  aria-label="Remove CRM association"
                >
                  <Delete01Icon className="h-3 w-3" />
                </button>
              ) : null}
            </div>
          );
        })}
      </AssociationsRailSection>

      {!excludeDocs && (
        <>
          <div className="my-4 h-px bg-border/60" />
          <AssociationsRailSection
            title="Docs"
            count={docs.length}
            emptyState="No linked docs"
            expanded={expandedSections.docs}
            onToggle={() => setExpandedSections((current) => ({ ...current, docs: !current.docs }))}
            onAdd={() => setPickerSection('docs')}
          >
            {visibleDocs.map((item) => (
              <div
                key={`${item.object_type}-${item.object_id}`}
                className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-muted/40"
              >
                <button
                  type="button"
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                  onClick={() => handleNavigate(item)}
                >
                  <File01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <span className="truncate font-medium">{item.title}</span>
                  {item.display_id && (
                    <span className="ml-auto shrink-0 text-muted-foreground">{item.display_id}</span>
                  )}
                </button>
                {item.association_id ? (
                  <button
                    type="button"
                    className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                    onClick={() => deleteDocAssociation.mutate(item.association_id!)}
                    aria-label="Remove document association"
                  >
                    <Delete01Icon className="h-3 w-3" />
                  </button>
                ) : null}
              </div>
            ))}
          </AssociationsRailSection>
        </>
      )}

      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle className="text-sm">Link {pickerTitle}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={pickerPlaceholder}
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="-mx-1 max-h-80 overflow-y-auto px-1">
              {searching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}

              {!searching && pickerSection === 'support' && conversationResults.map((conversation) => (
                <button
                  key={conversation.id}
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent"
                  onClick={() => handleAddSupport(conversation.id)}
                >
                  <span className="min-w-0 flex-1">{truncateText(conversation.subject, 60)}</span>
                  <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
                    C-{conversation.display_id}
                  </span>
                </button>
              ))}

              {!searching && pickerSection === 'crm' && crmResults.map((result) => (
                <button
                  key={`${result.type}-${result.id}`}
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent"
                  onClick={() => handleAddCRM(result.type as CRMObjectType, result.id)}
                >
                  {(() => {
                    const CRMIcon = crmIconMap[result.type as keyof typeof crmIconMap] ?? Building03Icon;
                    return <CRMIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />;
                  })()}
                  <span className="min-w-0 flex-1">{truncateText(result.name, 60)}</span>
                </button>
              ))}

              {!searching && pickerSection === 'docs' && docResults.map((doc) => (
                <button
                  key={doc.id}
                  type="button"
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none transition-colors hover:bg-accent focus-visible:bg-accent"
                  onClick={() => handleAddDoc(doc.id)}
                >
                  <File01Icon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                  <span className="min-w-0 flex-1">{truncateText(doc.name, 60)}</span>
                </button>
              ))}

              {!searching && pickerSection === 'support' && conversationResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && pickerSection !== 'support' && query.trim().length >= 2 &&
                ((pickerSection === 'crm' && crmResults.length === 0) ||
                 (pickerSection === 'docs' && docResults.length === 0)) && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && pickerSection !== 'support' && query.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <DocumentPreviewDialog
        workspaceId={workspaceId}
        slug={slug}
        docId={previewDocId}
        open={!!previewDocId}
        onOpenChange={(open) => {
          if (!open) setPreviewDocId(null);
        }}
      />
    </div>
  );
}
