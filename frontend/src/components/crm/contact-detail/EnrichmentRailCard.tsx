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
      detail: { query: enrichmentPromptFor(objectType) },
    }));
  };

  const buttonLabel = objectType === 'contact' ? 'Enrich contact' : 'Enrich record';

  if (results.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-md border border-dashed border-border/70 px-4 py-5 text-center">
        <p className="text-sm font-semibold">Pull from the web</p>
        <p className="text-xs text-muted-foreground">
          Use a sub-agent to research and update safe CRM fields.
        </p>
        <Button size="sm" className="mt-1 gap-1.5" onClick={handleEnrich}>
          <AskAgentAvatar plateStyle="feather" className="h-6 w-6" />
          {buttonLabel}
        </Button>
      </div>
    );
  }

  return (
    <EnrichmentHistoryList results={results} compact />
  );
}

function enrichmentPromptFor(objectType: CRMObjectType) {
  switch (objectType) {
    case 'contact':
      return 'find info about this contact and update contact and company';
    case 'company':
      return 'find info about this company and update company';
    case 'deal':
      return 'find info about this deal and update CRM context';
    default:
      return 'find info about this CRM record and update it';
  }
}
