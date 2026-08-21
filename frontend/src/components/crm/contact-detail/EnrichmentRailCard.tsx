import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { Button } from '@/components/ui/button';
import { useEnrichments } from '@/hooks/queries/useCRM';
import type { CRMEnrichmentResult, CRMObjectType } from '@/lib/crmTypes';
import { EnrichmentHistoryList, isVisibleEnrichmentResult } from '@/components/crm/EnrichmentSummary';
import { cn } from '@/lib/utils';

interface EnrichmentRailCardProps {
  workspaceId: string;
  objectType: CRMObjectType;
  objectId: string;
  presentation?: 'default' | 'borderless';
}

export function EnrichmentRailCard({ workspaceId, objectType, objectId, presentation = 'default' }: EnrichmentRailCardProps) {
  const { data } = useEnrichments(workspaceId, { object_type: objectType, object_id: objectId });
  const borderless = presentation === 'borderless';

  const results = ((data?.data ?? []) as CRMEnrichmentResult[]).filter(isVisibleEnrichmentResult);

  const handleEnrich = () => {
    window.dispatchEvent(new CustomEvent('helpin:ask-agents', {
      detail: { query: enrichmentPromptFor(objectType) },
    }));
  };

  const buttonLabel = objectType === 'contact' ? 'Enrich contact' : objectType === 'company' ? 'Enrich company' : 'Enrich record';

  if (results.length === 0) {
    return (
      <div className={cn(
        borderless
          ? '-mx-4 mt-4 border-y border-border/60 px-4 py-4 sm:-mx-6 sm:px-6 lg:-mx-5 lg:px-5'
          : 'flex flex-col items-center gap-2 rounded-md border border-dashed border-border/70 px-4 py-5 text-center',
      )}>
        {borderless ? (
          <>
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">Enrichment</h3>
              <Button type="button" variant="ghost" size="sm" className="h-7 gap-1.5 px-2 text-xs text-muted-foreground" onClick={handleEnrich}>
                <AskAgentAvatar plateStyle="feather" className="h-5 w-5" />
                Enrich
              </Button>
            </div>
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              Research trusted public sources and fill missing company details with source links.
            </p>
          </>
        ) : (
          <>
            <p className="text-sm font-semibold">Pull from the web</p>
            <p className="text-xs text-muted-foreground">
              Research trusted public sources and fill missing CRM details with source links.
            </p>
            <Button size="sm" className="mt-1 gap-1.5" onClick={handleEnrich}>
              <AskAgentAvatar plateStyle="feather" className="h-6 w-6" />
              {buttonLabel}
            </Button>
          </>
        )}
      </div>
    );
  }

  return (
    <div className={cn(
      borderless
        ? '-mx-4 mt-4 border-y border-border/60 px-4 py-4 sm:-mx-6 sm:px-6 lg:-mx-5 lg:px-5'
        : 'space-y-2.5',
    )}>
      <div className="flex items-center justify-between gap-2">
        <span className={cn(borderless ? 'text-xs font-semibold uppercase tracking-wide text-foreground/70' : 'text-[11px] text-muted-foreground')}>
          {borderless ? 'Enrichment' : 'Sourced CRM data'}
        </span>
        <Button type="button" variant={borderless ? 'ghost' : 'outline'} size="sm" className="h-7 gap-1 px-2 text-[11px]" onClick={handleEnrich}>
          <AskAgentAvatar plateStyle="feather" className="h-5 w-5" />
          Refresh
        </Button>
      </div>
      <div className={cn(borderless && 'mt-2')}>
        <EnrichmentHistoryList results={results} compact workspaceId={workspaceId} presentation={presentation} />
      </div>
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
