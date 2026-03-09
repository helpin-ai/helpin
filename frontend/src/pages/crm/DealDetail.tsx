import { useNavigate } from '@tanstack/react-router';
import { ArrowLeft, DollarSign, Calendar } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDeal, useDealActivities, useDealAssociations } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { DealHealthScore } from '@/components/crm/DealHealthScore';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { useTitle } from '@/hooks/useTitle';

export function DealDetailPage({ dealId }: { dealId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: deal, isLoading } = useDeal(wsId, dealId);
  const { data: activitiesData } = useDealActivities(wsId, dealId);
  const { data: associations } = useDealAssociations(wsId, dealId);

  useTitle(deal?.name ?? 'Deal');

  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading...</div>;
  }

  if (!deal) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Deal not found</div>;
  }

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      <Button variant="ghost" size="sm" className="mb-4" onClick={() => navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } })}>
        <ArrowLeft className="mr-1 h-4 w-4" />
        Back to Deals
      </Button>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="md:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs text-muted-foreground">{deal.display_id}</p>
                  <CardTitle className="text-xl">{deal.name}</CardTitle>
                </div>
                {deal.stage && (
                  <Badge variant="outline">{deal.stage.name}</Badge>
                )}
              </div>
            </CardHeader>
            <CardContent className="space-y-3">
              {deal.amount != null && (
                <div className="flex items-center gap-2 text-sm">
                  <DollarSign className="h-4 w-4 text-muted-foreground" />
                  <span>{deal.currency} {deal.amount.toLocaleString()}</span>
                </div>
              )}
              {deal.close_date && (
                <div className="flex items-center gap-2 text-sm">
                  <Calendar className="h-4 w-4 text-muted-foreground" />
                  <span>Close date: {new Date(deal.close_date).toLocaleDateString()}</span>
                </div>
              )}
              {deal.probability != null && (
                <div className="text-sm">
                  <span className="text-muted-foreground">Probability:</span> {deal.probability}%
                </div>
              )}
              {deal.pipeline && (
                <div className="text-sm">
                  <span className="text-muted-foreground">Pipeline:</span> {deal.pipeline.name}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Activity</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={activitiesData?.data ?? []} />
            </CardContent>
          </Card>

          <EmailTimeline workspaceId={wsId} dealId={dealId} />
        </div>

        <div className="space-y-6">
          <DealHealthScore workspaceId={wsId} dealId={dealId} />
          <BuyerSignals workspaceId={wsId} dealId={dealId} />

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Associations</CardTitle>
            </CardHeader>
            <CardContent>
              {(!associations || associations.length === 0) ? (
                <p className="text-sm text-muted-foreground">No associations yet</p>
              ) : (
                <ul className="space-y-2">
                  {associations.map((assoc) => (
                    <li key={assoc.id} className="text-sm">
                      <span className="capitalize">{assoc.from_object_type === 'deal' ? assoc.to_object_type : assoc.from_object_type}</span>
                      {assoc.association_label && <span className="text-muted-foreground"> ({assoc.association_label})</span>}
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
