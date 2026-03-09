import { createFileRoute } from '@tanstack/react-router';
import { ContactsPage } from '@/pages/crm/Contacts';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/contacts/')({
  component: ContactsRoute,
});

function ContactsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <ContactsPage />
    </div>
  );
}
