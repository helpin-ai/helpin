import { useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Globe, Users, DollarSign } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCompany, useCompanyActivities, useCompanyAssociations } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { useTitle } from '@/hooks/useTitle';

export function CompanyDetailPage({ companyId }: { companyId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: company, isLoading } = useCompany(wsId, companyId);
  const { data: activitiesData } = useCompanyActivities(wsId, companyId);
  const { data: associations } = useCompanyAssociations(wsId, companyId);

  useTitle(company?.name ?? 'Company');

  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading...</div>;
  }

  if (!company) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Company not found</div>;
  }

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      <Button variant="ghost" size="sm" className="mb-4" onClick={() => navigate({ to: '/w/$slug/crm/companies', params: { slug: wsSlug } })}>
        <ArrowLeft className="mr-1 h-4 w-4" />
        Back to Companies
      </Button>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="md:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <p className="text-xs text-muted-foreground">{company.display_id}</p>
              <CardTitle className="text-xl">{company.name}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {company.domain && (
                <div className="flex items-center gap-2 text-sm">
                  <Globe className="h-4 w-4 text-muted-foreground" />
                  <span>{company.domain}</span>
                </div>
              )}
              {company.industry && (
                <div className="text-sm">
                  <span className="text-muted-foreground">Industry:</span> {company.industry}
                </div>
              )}
              {company.employee_count != null && (
                <div className="flex items-center gap-2 text-sm">
                  <Users className="h-4 w-4 text-muted-foreground" />
                  <span>{company.employee_count} employees</span>
                </div>
              )}
              {company.annual_revenue != null && (
                <div className="flex items-center gap-2 text-sm">
                  <DollarSign className="h-4 w-4 text-muted-foreground" />
                  <span>${company.annual_revenue.toLocaleString()} annual revenue</span>
                </div>
              )}
              {company.description && (
                <p className="text-sm text-muted-foreground">{company.description}</p>
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
        </div>

        <div className="space-y-6">
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
                      <span className="capitalize">{assoc.from_object_type === 'company' ? assoc.to_object_type : assoc.from_object_type}</span>
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
