import { format } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import type { CRMContact } from '@/lib/crmTypes';

interface ContactsTableProps {
  contacts: CRMContact[];
  total: number;
  isLoading: boolean;
  onRowClick: (id: string) => void;
}

const lifecycleColors: Record<string, string> = {
  subscriber: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  lead: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  marketing_qualified: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  sales_qualified: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
  opportunity: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  customer: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  evangelist: 'bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-300',
};

export function ContactsTable({ contacts, total, isLoading, onRowClick }: ContactsTableProps) {
  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading contacts...</div>;
  }

  if (contacts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center">
        <p className="text-muted-foreground">No contacts found</p>
        <p className="mt-1 text-sm text-muted-foreground/70">Create your first contact to get started</p>
      </div>
    );
  }

  return (
    <div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-2 pr-4 font-medium">Name</th>
            <th className="pb-2 pr-4 font-medium">Email</th>
            <th className="pb-2 pr-4 font-medium">Stage</th>
            <th className="pb-2 pr-4 font-medium">Status</th>
            <th className="pb-2 font-medium">Created</th>
          </tr>
        </thead>
        <tbody>
          {contacts.map((contact) => (
            <tr
              key={contact.id}
              className="cursor-pointer border-b transition-colors hover:bg-muted/50"
              onClick={() => onRowClick(contact.id)}
            >
              <td className="py-2.5 pr-4">
                <div className="font-medium">{contact.first_name} {contact.last_name}</div>
                <div className="text-xs text-muted-foreground">{contact.display_id}</div>
              </td>
              <td className="py-2.5 pr-4 text-muted-foreground">{contact.email ?? '-'}</td>
              <td className="py-2.5 pr-4">
                <Badge variant="outline" className={lifecycleColors[contact.lifecycle_stage] ?? ''}>
                  {contact.lifecycle_stage.replace(/_/g, ' ')}
                </Badge>
              </td>
              <td className="py-2.5 pr-4 capitalize">{contact.lead_status.replace(/_/g, ' ')}</td>
              <td className="py-2.5 text-muted-foreground">
                {format(new Date(contact.created_at), 'MMM d, yyyy')}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-2 text-xs text-muted-foreground">{total} total contacts</p>
    </div>
  );
}
