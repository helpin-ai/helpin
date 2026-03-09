import { useMemo } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { SquareKanban, Plus } from 'lucide-react';
import type { CRMDeal, CRMPipeline } from '@/lib/crmTypes';

interface DealBoardProps {
  deals: CRMDeal[];
  pipeline?: CRMPipeline;
  onDealClick: (id: string) => void;
  onCreateClick?: () => void;
}

export function DealBoard({ deals, pipeline, onDealClick, onCreateClick }: DealBoardProps) {
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
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <SquareKanban className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No pipeline configured</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Create a pipeline in settings to use the board view
        </p>
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

            <div className="flex min-h-[200px] flex-col gap-2 rounded-lg bg-muted/30 p-2">
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
                    {deal.probability != null && (
                      <div className="mt-1.5 flex items-center gap-1.5">
                        <div className="h-1 flex-1 overflow-hidden rounded-full bg-muted">
                          <div
                            className="h-full rounded-full bg-primary/60"
                            style={{ width: `${deal.probability}%` }}
                          />
                        </div>
                        <span className="text-[10px] text-muted-foreground">{deal.probability}%</span>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
              {stageDeals.length === 0 && (
                <div className="flex flex-col items-center justify-center p-4 text-center">
                  <p className="text-xs text-muted-foreground">No deals in this stage</p>
                  {onCreateClick && (
                    <Button variant="ghost" size="sm" className="mt-2 text-xs" onClick={onCreateClick}>
                      <Plus className="mr-1 h-3 w-3" /> Add deal
                    </Button>
                  )}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
