import { useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Mail, Phone, Briefcase } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useContact, useContactActivities, useContactAssociations } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EnrichmentCard } from '@/components/crm/EnrichmentCard';
import { useTitle } from '@/hooks/useTitle';

export function ContactDetailPage({ contactId }: { contactId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: contact, isLoading } = useContact(wsId, contactId);
  const { data: activitiesData } = useContactActivities(wsId, contactId);
  const { data: associations } = useContactAssociations(wsId, contactId);

  useTitle(contact ? `${contact.first_name} ${contact.last_name ?? ''}` : 'Contact');

  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading...</div>;
  }

  if (!contact) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Contact not found</div>;
  }

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      <Button variant="ghost" size="sm" className="mb-4" onClick={() => navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } })}>
        <ArrowLeft className="mr-1 h-4 w-4" />
        Back to Contacts
      </Button>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="md:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs text-muted-foreground">{contact.display_id}</p>
                  <CardTitle className="text-xl">
                    {contact.first_name} {contact.last_name}
                  </CardTitle>
                </div>
                <div className="flex gap-2">
                  <Badge variant="outline">{contact.lifecycle_stage.replace(/_/g, ' ')}</Badge>
                  <Badge variant="secondary">{contact.lead_status.replace(/_/g, ' ')}</Badge>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-3">
              {contact.email && (
                <div className="flex items-center gap-2 text-sm">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  <span>{contact.email}</span>
                </div>
              )}
              {contact.phone && (
                <div className="flex items-center gap-2 text-sm">
                  <Phone className="h-4 w-4 text-muted-foreground" />
                  <span>{contact.phone}</span>
                </div>
              )}
              {contact.job_title && (
                <div className="flex items-center gap-2 text-sm">
                  <Briefcase className="h-4 w-4 text-muted-foreground" />
                  <span>{contact.job_title}</span>
                </div>
              )}
              {contact.source && (
                <div className="text-sm text-muted-foreground">
                  Source: {contact.source}
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

          <EmailTimeline workspaceId={wsId} contactId={contactId} />
        </div>

        <div className="space-y-6">
          <EnrichmentCard workspaceId={wsId} objectType="contact" objectId={contactId} />
          <BuyerSignals workspaceId={wsId} contactId={contactId} />

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
                      <span className="capitalize">{assoc.from_object_type === 'contact' ? assoc.to_object_type : assoc.from_object_type}</span>
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
