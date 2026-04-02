import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  Building2,
  FileText,
  Loader2,
  MessageSquareText,
  Search,
} from 'lucide-react';

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
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMSearchResult, CRMObjectType } from '@/lib/crmTypes';
import type {
  AssociationObjectSummary,
  GroupedAssociations,
  SupportConversation,
} from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { CompactChip } from '@/components/ui/compact-chip';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

type AssociationsObjectType = 'task' | 'epic';
type SectionKey = 'support' | 'crm' | 'docs';

interface AssociationsPanelProps {
  workspaceId: string;
  objectType: AssociationsObjectType;
  objectId: string;
  className?: string;
  includeTaskRelationships?: boolean;
}

export function AssociationsPanel({
  workspaceId,
  objectType,
  objectId,
  className,
}: AssociationsPanelProps) {
  const navigate = useNavigate();
  const slug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');

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
      navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug, docId: id } } as any);
    }
  };

  const [pickerSection, setPickerSection] = useState<SectionKey | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [docResults, setDocResults] = useState<SearchResult[]>([]);
  const [conversationResults, setConversationResults] = useState<SupportConversation[]>([]);

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
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading...
      </div>
    );
  }

  const supportConversations = data?.support_conversations ?? [];
  const crmRecords = data?.crm_records ?? [];
  const docs = data?.docs ?? [];

  const pickerTitle =
    pickerSection === 'support' ? 'Support Conversation' :
    pickerSection === 'crm' ? 'CRM Record' :
    'Document';

  const pickerPlaceholder =
    pickerSection === 'support' ? 'Filter conversations by subject or ID' :
    pickerSection === 'crm' ? 'Search contacts, companies, or deals' :
    'Search documents';

  return (
    <div className={className}>
      <CollapsibleSection
        title="Support"
        icon={MessageSquareText}
        count={supportConversations.length}
        defaultOpen={supportConversations.length > 0}
        onAdd={() => setPickerSection('support')}
      >
        {supportConversations.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked support conversations</p>
        ) : (
          supportConversations.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              title={item.title}
              displayId={item.display_id}
              onClick={() => handleNavigate(item)}
              onRemove={item.association_id ? () => deleteAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="CRM"
        icon={Building2}
        count={crmRecords.length}
        defaultOpen={crmRecords.length > 0}
        onAdd={() => setPickerSection('crm')}
      >
        {crmRecords.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked CRM records</p>
        ) : (
          crmRecords.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              title={item.title}
              displayId={item.display_id}
              onClick={() => handleNavigate(item)}
              onRemove={item.association_id ? () => deleteAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      <CollapsibleSection
        title="Docs"
        icon={FileText}
        count={docs.length}
        defaultOpen={docs.length > 0}
        onAdd={() => setPickerSection('docs')}
      >
        {docs.length === 0 ? (
          <p className="text-[11px] text-muted-foreground italic py-1">No linked docs</p>
        ) : (
          docs.map((item) => (
            <CompactChip
              key={`${item.object_type}-${item.object_id}`}
              title={item.title}
              displayId={item.display_id}
              onClick={() => handleNavigate(item)}
              onRemove={item.association_id ? () => deleteDocAssociation.mutate(item.association_id!) : undefined}
            />
          ))
        )}
      </CollapsibleSection>

      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">Link {pickerTitle}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={pickerPlaceholder}
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {searching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}

              {!searching && pickerSection === 'support' && conversationResults.map((conversation) => (
                <button
                  key={conversation.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddSupport(conversation.id)}
                >
                  <div className="flex items-center gap-2">
                    <MessageSquareText className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{conversation.subject}</span>
                    <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                      C-{conversation.display_id}
                    </Badge>
                  </div>
                </button>
              ))}

              {!searching && pickerSection === 'crm' && crmResults.map((result) => (
                <button
                  key={`${result.type}-${result.id}`}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddCRM(result.type as CRMObjectType, result.id)}
                >
                  <div className="flex items-center gap-2">
                    <Building2 className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{result.name}</span>
                  </div>
                </button>
              ))}

              {!searching && pickerSection === 'docs' && docResults.map((doc) => (
                <button
                  key={doc.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddDoc(doc.id)}
                >
                  <div className="flex items-center gap-2">
                    <FileText className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{doc.name}</span>
                  </div>
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
    </div>
  );
}
