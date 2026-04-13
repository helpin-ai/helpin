import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/contacts/$contactId')({
  component: ContactDetailRoute,
});

const ContactDetailPage = lazyRouteComponent(() => import('@/pages/crm/ContactDetail'), 'ContactDetailPage');

function ContactDetailRoute() {
  const { contactId } = Route.useParams();
  return (
    <div className="h-full overflow-hidden">
      <ContactDetailPage contactId={contactId} />
    </div>
  );
}
