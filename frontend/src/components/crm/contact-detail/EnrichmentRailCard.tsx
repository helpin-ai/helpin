import { Loading01Icon, SparklesIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useEnrichments, useCreateEnrichment } from '@/hooks/queries/useCRM';
import type { CRMEnrichmentResult, CRMObjectType } from '@/lib/crmTypes';

interface EnrichmentRailCardProps {
  workspaceId: string;
  objectType: CRMObjectType;
  objectId: string;
}

export function EnrichmentRailCard({ workspaceId, objectType, objectId }: EnrichmentRailCardProps) {
  const { data } = useEnrichments(workspaceId, { object_type: objectType, object_id: objectId });
  const createEnrichment = useCreateEnrichment(workspaceId);

  const results = (data?.data ?? []) as CRMEnrichmentResult[];
  const pending = createEnrichment.isPending;

  const handleEnrich = () => {
    createEnrichment.mutate({
      workspace_id: workspaceId,
      object_type: objectType,
      object_id: objectId,
      source: 'manual',
      data: {},
      confidence: 0,
    });
  };

  if (results.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 rounded-md border border-dashed border-border/70 px-4 py-5 text-center">
        <p className="text-sm font-semibold">Pull from the web</p>
        <p className="text-xs text-muted-foreground">
          Auto-fill title, company, LinkedIn and more.
        </p>
        <Button size="sm" className="mt-1 gap-1.5" onClick={handleEnrich} disabled={pending}>
          {pending ? (
            <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <SparklesIcon className="h-3.5 w-3.5" />
          )}
          {pending ? 'Enriching…' : 'Enrich contact'}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {results.map((result) => (
        <div key={result.id} className="rounded-md border border-border/60 p-2.5">
          <div className="flex items-center justify-between">
            <Badge variant="outline" className="text-[10px]">
              {result.source}
            </Badge>
            <span className="text-[10px] text-muted-foreground">
              {Math.round(result.confidence * 100)}%
            </span>
          </div>
          <div className="mt-1.5 space-y-1">
            {Object.entries(result.data).map(([key, value]) => (
              <div key={key} className="flex items-start justify-between gap-2 text-[11px]">
                <span className="shrink-0 text-muted-foreground">{key}</span>
                <span className="truncate text-right font-medium">{String(value)}</span>
              </div>
            ))}
          </div>
        </div>
      ))}
      <Button
        size="sm"
        variant="outline"
        className="w-full gap-1.5"
        onClick={handleEnrich}
        disabled={pending}
      >
        {pending ? (
          <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        ) : (
          <SparklesIcon className="h-3.5 w-3.5" />
        )}
        {pending ? 'Enriching…' : 'Re-enrich'}
      </Button>
    </div>
  );
}
