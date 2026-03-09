import { createFileRoute } from '@tanstack/react-router';
import { ContactDetailPage } from '@/pages/crm/ContactDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/contacts/$contactId')({
  component: ContactDetailRoute,
});

function ContactDetailRoute() {
  const { contactId } = Route.useParams();
  return (
    <div className="h-full overflow-auto pt-4 md:pt-6">
      <ContactDetailPage contactId={contactId} />
    </div>
  );
}
