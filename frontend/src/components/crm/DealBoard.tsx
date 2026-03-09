import { useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import type { CRMDeal, CRMPipeline } from '@/lib/crmTypes';

interface DealBoardProps {
  deals: CRMDeal[];
  pipeline?: CRMPipeline;
  onDealClick: (id: string) => void;
}

export function DealBoard({ deals, pipeline, onDealClick }: DealBoardProps) {
  const stages = useMemo(() => {
    if (!pipeline?.stages) return [];
    return [...pipeline.stages].sort((a, b) => a.position - b.position);
  }, [pipeline?.stages]);

  const dealsByStage = useMemo(() => {
    const map = new Map<string, CRMDeal[]>();
    for (const stage of stages) {
      map.set(stage.id, []);
    }
    for (const deal of deals) {
      const existing = map.get(deal.stage_id);
      if (existing) {
        existing.push(deal);
      }
    }
    return map;
  }, [deals, stages]);

  if (!pipeline || stages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center">
        <p className="text-muted-foreground">No pipeline configured</p>
        <p className="mt-1 text-sm text-muted-foreground/70">Create a pipeline in settings to use the board view</p>
      </div>
    );
  }

  return (
    <div className="flex gap-4 overflow-x-auto pb-4">
      {stages.map((stage) => {
        const stageDeals = dealsByStage.get(stage.id) ?? [];
        const stageTotal = stageDeals.reduce((sum, d) => sum + (d.amount ?? 0), 0);

        return (
          <div key={stage.id} className="flex w-72 shrink-0 flex-col">
            <div className="mb-2 flex items-center justify-between px-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{stage.name}</span>
                <Badge variant="secondary" className="text-xs">{stageDeals.length}</Badge>
              </div>
              {stageTotal > 0 && (
                <span className="text-xs text-muted-foreground">
                  ${stageTotal.toLocaleString()}
                </span>
              )}
            </div>

            <div className="flex flex-col gap-2 rounded-lg bg-muted/30 p-2 min-h-[200px]">
              {stageDeals.map((deal) => (
                <Card
                  key={deal.id}
                  className="cursor-pointer transition-shadow hover:shadow-md"
                  onClick={() => onDealClick(deal.id)}
                >
                  <CardContent className="p-3">
                    <p className="text-sm font-medium leading-tight">{deal.name}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">{deal.display_id}</p>
                    {deal.amount != null && (
                      <p className="mt-1.5 text-sm font-medium text-green-600 dark:text-green-400">
                        {deal.currency} {deal.amount.toLocaleString()}
                      </p>
                    )}
                    {deal.close_date && (
                      <p className="mt-1 text-xs text-muted-foreground">
                        Close: {new Date(deal.close_date).toLocaleDateString()}
                      </p>
                    )}
                  </CardContent>
                </Card>
              ))}
              {stageDeals.length === 0 && (
                <div className="flex items-center justify-center p-4 text-xs text-muted-foreground">
                  No deals
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
