import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { Button } from '@/components/ui/button';
import { useEnrichments } from '@/hooks/queries/useCRM';
import type { CRMEnrichmentResult, CRMObjectType } from '@/lib/crmTypes';
import { EnrichmentHistoryList, isVisibleEnrichmentResult } from '@/components/crm/EnrichmentSummary';

interface EnrichmentRailCardProps {
  workspaceId: string;
  objectType: CRMObjectType;
  objectId: string;
}

export function EnrichmentRailCard({ workspaceId, objectType, objectId }: EnrichmentRailCardProps) {
  const { data } = useEnrichments(workspaceId, { object_type: objectType, object_id: objectId });

  const results = ((data?.data ?? []) as CRMEnrichmentResult[]).filter(isVisibleEnrichmentResult);

  const handleEnrich = () => {
    window.dispatchEvent(new CustomEvent('helpin:ask-agents', {
      detail: { intent: 'new_chat', query: enrichmentPromptFor(objectType) },
    }));
  };

  const buttonLabel = objectType === 'contact' ? 'Enrich contact' : objectType === 'company' ? 'Enrich company' : 'Enrich record';

  if (results.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-md border border-dashed border-border/70 px-4 py-5 text-center">
        <p className="text-sm font-semibold">Pull from the web</p>
        <p className="text-xs text-muted-foreground">
          Research trusted public sources and fill missing CRM details with source links.
        </p>
        <Button size="sm" className="mt-1 gap-1.5" onClick={handleEnrich}>
          <AskAgentAvatar plateStyle="feather" className="h-6 w-6" />
          {buttonLabel}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] text-muted-foreground">Sourced CRM data</span>
        <Button type="button" variant="outline" size="sm" className="h-7 gap-1 px-2 text-[11px]" onClick={handleEnrich}>
          <AskAgentAvatar plateStyle="feather" className="h-5 w-5" />
          Refresh
        </Button>
      </div>
      <EnrichmentHistoryList results={results} compact workspaceId={workspaceId} />
    </div>
  );
}

function enrichmentPromptFor(objectType: CRMObjectType) {
  switch (objectType) {
    case 'contact':
      return 'Enrich this contact from reliable public sources. Fill missing CRM fields, keep existing values unless I approve a suggested replacement, include source links, and enrich or link the company when confidently identified.';
    case 'company':
      return 'Enrich this company from reliable public sources. Fill missing CRM fields, keep existing values unless I approve a suggested replacement, and include source links for every field.';
    case 'deal':
      return 'find info about this deal and update CRM context';
    default:
      return 'find info about this CRM record and update it';
  }
}
