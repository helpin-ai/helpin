import { DatabaseIcon } from '@/lib/icons';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useEnrichments } from '@/hooks/queries/useCRM';
import type { CRMEnrichmentResult, CRMObjectType } from '@/lib/crmTypes';
import { EnrichmentHistoryList, isVisibleEnrichmentResult } from '@/components/crm/EnrichmentSummary';

interface EnrichmentCardProps {
  workspaceId: string;
  objectType: CRMObjectType;
  objectId: string;
}

export function EnrichmentCard({ workspaceId, objectType, objectId }: EnrichmentCardProps) {
  const { data } = useEnrichments(workspaceId, { object_type: objectType, object_id: objectId });

  const results = ((data?.data ?? []) as CRMEnrichmentResult[]).filter(isVisibleEnrichmentResult);

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <DatabaseIcon className="h-4 w-4" />
            Enrichment Data
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {results.length === 0 ? (
          <p className="text-sm text-muted-foreground">No enrichment data available.</p>
        ) : (
          <EnrichmentHistoryList results={results} />
        )}
      </CardContent>
    </Card>
  );
}
