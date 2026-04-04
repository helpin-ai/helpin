import { DatabaseIcon, ArrowReloadHorizontalIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useEnrichments, useCreateEnrichment } from '@/hooks/queries/useCRM';
import type { CRMEnrichmentResult, CRMObjectType } from '@/lib/crmTypes';

interface EnrichmentCardProps {
  workspaceId: string;
  objectType: CRMObjectType;
  objectId: string;
}

export function EnrichmentCard({ workspaceId, objectType, objectId }: EnrichmentCardProps) {
  const { data } = useEnrichments(workspaceId, { object_type: objectType, object_id: objectId });
  const createEnrichment = useCreateEnrichment(workspaceId);

  const results = (data?.data ?? []) as CRMEnrichmentResult[];

  const handleReEnrich = () => {
    createEnrichment.mutate({
      workspace_id: workspaceId,
      object_type: objectType,
      object_id: objectId,
      source: 'manual',
      data: {},
      confidence: 0,
    });
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <DatabaseIcon className="h-4 w-4" />
            Enrichment Data
          </CardTitle>
          <Button variant="outline" size="sm" onClick={handleReEnrich} disabled={createEnrichment.isPending}>
            <ArrowReloadHorizontalIcon className="mr-1 h-3 w-3" />
            Re-enrich
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {results.length === 0 ? (
          <p className="text-sm text-muted-foreground">No enrichment data available.</p>
        ) : (
          <div className="space-y-3">
            {results.map((result) => (
              <div key={result.id} className="rounded-md border p-3">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-xs">{result.source}</Badge>
                  <span className="text-xs text-muted-foreground">
                    Confidence: {Math.round(result.confidence * 100)}%
                  </span>
                </div>
                <div className="mt-2 space-y-1">
                  {Object.entries(result.data).map(([key, value]) => (
                    <div key={key} className="flex items-center justify-between text-xs">
                      <span className="text-muted-foreground">{key}</span>
                      <span className="font-medium">{String(value)}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
