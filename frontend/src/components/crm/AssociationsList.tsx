import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  Building2,
  DollarSign,
  FileText,
  GitBranch,
  Hexagon,
  Loader2,
  MessageSquareText,
  Search,
  Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Favicon } from '@/components/ui/favicon';
import { Input } from '@/components/ui/input';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { CompactChip } from '@/components/ui/compact-chip';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useCreateAssociation, useDeleteAssociation } from '@/hooks/queries/useCRM';
import { crmSearchService } from '@/lib/services/crmService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import { supportService } from '@/lib/services/supportService';
import type { CRMAssociationEnriched, CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import type { SupportConversation } from '@/lib/pmTypes';

interface AssociationsListProps {
  workspaceId: string;
  slug: string;
  associations: CRMAssociationEnriched[];
  currentObjectType: CRMObjectType;
  currentObjectId: string;
  onAssociationRemoved?: () => void;
}

type SectionType = CRMObjectType;

const sectionConfig: Record<string, { title: string; icon: React.ElementType }> = {
  contact: { title: 'Contacts', icon: Users },
  company: { title: 'Companies', icon: Building2 },
  deal: { title: 'Deals', icon: DollarSign },
  epic: { title: 'Epics', icon: Hexagon },
  story: { title: 'Stories', icon: GitBranch },
  support_conversation: { title: 'Support', icon: MessageSquareText },
};

export function AssociationsList({
  workspaceId,
  slug,
  associations,
  currentObjectType,
  currentObjectId,
  onAssociationRemoved,
}: AssociationsListProps) {
  const navigate = useNavigate();
  const createAssociation = useCreateAssociation(workspaceId);
  const deleteAssociation = useDeleteAssociation(workspaceId);
  const [removeId, setRemoveId] = useState<string | null>(null);
  const [pickerSection, setPickerSection] = useState<SectionType | null>(null);
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [pmResults, setPMResults] = useState<SearchResult[]>([]);
  const [conversationResults, setConversationResults] = useState<SupportConversation[]>([]);

  const grouped = useMemo(() => {
    const groups: Record<string, Array<CRMAssociationEnriched & { linkedType: CRMObjectType; linkedId: string }>> = {};
    for (const assoc of associations) {
      const isFrom = assoc.from_object_type === currentObjectType && assoc.from_object_id === currentObjectId;
      const linkedType = isFrom ? assoc.to_object_type : assoc.from_object_type;
      const linkedId = isFrom ? assoc.to_object_id : assoc.from_object_id;
      (groups[linkedType] ??= []).push({ ...assoc, linkedType, linkedId });
    }
    return groups;
  }, [associations, currentObjectType, currentObjectId]);

  const handleNavigate = (type: CRMObjectType, id: string) => {
    const routes: Partial<Record<CRMObjectType, { to: string; params: Record<string, string> }>> = {
      contact: { to: '/w/$slug/crm/contacts/$contactId', params: { slug, contactId: id } },
      company: { to: '/w/$slug/crm/companies/$companyId', params: { slug, companyId: id } },
      deal: { to: '/w/$slug/crm/deals/$dealId', params: { slug, dealId: id } },
      epic: { to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: id } },
      story: { to: '/w/$slug/pm/stories/$storyId', params: { slug, storyId: id } },
      support_conversation: { to: '/w/$slug/support/$conversationId', params: { slug, conversationId: id } },
    };
    const route = routes[type];
    if (route) navigate(route as any);
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

  // Search logic for the add picker
  useEffect(() => {
    if (!pickerSection) {
      setQuery('');
      setCRMResults([]);
      setPMResults([]);
      setConversationResults([]);
      setSearching(false);
      return;
    }

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
          })
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
      } else if (pickerSection === 'epic' || pickerSection === 'story') {
        const response = await searchService.search(workspaceId, query.trim());
        const items = pickerSection === 'epic' ? (response.data?.epics ?? []) : (response.data?.stories ?? []);
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
    setPickerSection(null);
    onAssociationRemoved?.(); // triggers refetch
  };

  const sectionOrder: SectionType[] = ['contact', 'company', 'deal', 'epic', 'story', 'support_conversation'];
  const visibleSections = sectionOrder.filter((type) => (grouped[type]?.length ?? 0) > 0);

  const pickerConfig = pickerSection ? sectionConfig[pickerSection] : null;
  const pickerPlaceholder =
    pickerSection === 'support_conversation' ? 'Filter conversations by subject or ID' :
    pickerSection === 'epic' || pickerSection === 'story' ? `Search ${pickerSection}s by title or ID` :
    `Search ${pickerSection ? pickerSection + 's' : ''}`;

  const pickerIcon = pickerConfig?.icon ?? FileText;
  const PickerIcon = pickerIcon;

  return (
    <div>
      {visibleSections.map((type) => {
        const items = grouped[type]!;
        const config = sectionConfig[type];
        if (!config) return null;

        return (
          <CollapsibleSection
            key={type}
            title={config.title}
            icon={config.icon}
            count={items.length}
            defaultOpen={items.length > 0}
            onAdd={() => setPickerSection(type)}
          >
            {items.map((assoc) => (
              <CompactChip
                key={assoc.id}
                title={assoc.linked_object_name || assoc.linkedType}
                displayId={assoc.linked_object_display_id || undefined}
                onClick={() => handleNavigate(assoc.linkedType, assoc.linkedId)}
                onRemove={() => setRemoveId(assoc.id)}
              />
            ))}
          </CollapsibleSection>
        );
      })}

      {visibleSections.length === 0 && (
        <p className="text-[11px] text-muted-foreground italic py-3 px-3">No associations</p>
      )}

      <Dialog open={!!pickerSection} onOpenChange={(open) => { if (!open) setPickerSection(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">Link {pickerConfig?.title?.replace(/s$/, '') ?? ''}</DialogTitle>
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

              {!searching && pickerSection === 'support_conversation' && conversationResults.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAdd('support_conversation', c.id)}
                >
                  <div className="flex items-center gap-2">
                    <MessageSquareText className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{c.subject}</span>
                    <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                      C-{c.display_id}
                    </Badge>
                  </div>
                </button>
              ))}

              {!searching && (pickerSection === 'contact' || pickerSection === 'company' || pickerSection === 'deal') && crmResults.map((r) => (
                <button
                  key={`${r.type}-${r.id}`}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAdd(r.type as CRMObjectType, r.id)}
                >
                  <div className="flex items-center gap-2">
                    {r.type === 'company' ? (
                      <Favicon
                        src={'logo_url' in r.object ? r.object.logo_url : undefined}
                        url={'domain' in r.object ? r.object.domain : undefined}
                        name={r.name}
                        size={16}
                        className="h-3.5 w-3.5 rounded-sm border-none bg-transparent"
                        fallbackClassName="text-[7px]"
                      />
                    ) : (
                      <PickerIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    )}
                    <span className="font-medium truncate">{r.name}</span>
                  </div>
                </button>
              ))}

              {!searching && (pickerSection === 'epic' || pickerSection === 'story') && pmResults.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAdd(pickerSection!, r.id)}
                >
                  <div className="flex items-center gap-2">
                    <PickerIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{r.name}</span>
                    {r.display_id && (
                      <Badge variant="outline" className="h-5 px-1.5 text-[10px] shrink-0">
                        {r.display_id}
                      </Badge>
                    )}
                  </div>
                </button>
              ))}

              {!searching && pickerSection === 'support_conversation' && conversationResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && pickerSection !== 'support_conversation' && query.trim().length >= 2 &&
                ((pickerSection === 'contact' || pickerSection === 'company' || pickerSection === 'deal') && crmResults.length === 0 ||
                 (pickerSection === 'epic' || pickerSection === 'story') && pmResults.length === 0) && (
                <p className="py-4 text-sm text-muted-foreground text-center">No results found</p>
              )}
              {!searching && pickerSection !== 'support_conversation' && query.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </div>
          </div>
        </DialogContent>
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
