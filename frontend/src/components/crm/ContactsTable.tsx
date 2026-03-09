import { useMemo, useState } from 'react';
import { format } from 'date-fns';
import { ArrowUp, ArrowDown, ArrowUpDown, Users, Plus } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import type { CRMContact } from '@/lib/crmTypes';

interface ContactsTableProps {
  contacts: CRMContact[];
  total: number;
  isLoading: boolean;
  onRowClick: (id: string) => void;
  onCreateClick?: () => void;
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

type SortField = 'name' | 'email' | 'lifecycle_stage' | 'lead_status' | 'created_at';
type SortDir = 'asc' | 'desc';

function SortIcon({ field, sortField, sortDir }: { field: SortField; sortField: SortField; sortDir: SortDir }) {
  if (sortField !== field) return <ArrowUpDown className="h-3 w-3 opacity-50" />;
  return sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />;
}

const headers: { field: SortField; label: string }[] = [
  { field: 'name', label: 'Name' },
  { field: 'email', label: 'Email' },
  { field: 'lifecycle_stage', label: 'Stage' },
  { field: 'lead_status', label: 'Status' },
  { field: 'created_at', label: 'Created' },
];

export function ContactsTable({ contacts, total, isLoading, onRowClick, onCreateClick }: ContactsTableProps) {
  const [sortField, setSortField] = useState<SortField>('created_at');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortField(field);
      setSortDir('asc');
    }
  };

  const sorted = useMemo(() => {
    return [...contacts].sort((a, b) => {
      let av: string | number;
      let bv: string | number;
      if (sortField === 'name') {
        av = `${a.first_name} ${a.last_name ?? ''}`.toLowerCase();
        bv = `${b.first_name} ${b.last_name ?? ''}`.toLowerCase();
      } else {
        av = (a[sortField] as string) ?? '';
        bv = (b[sortField] as string) ?? '';
      }
      const cmp = typeof av === 'number' ? av - Number(bv) : String(av).localeCompare(String(bv));
      return sortDir === 'asc' ? cmp : -cmp;
    });
  }, [contacts, sortField, sortDir]);

  if (isLoading) {
    return (
      <div>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b text-left text-muted-foreground">
              {headers.map((h) => (
                <th key={h.field} className="pb-2 pr-4 font-medium">{h.label}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {Array.from({ length: 5 }).map((_, i) => (
              <tr key={i} className="border-b">
                <td className="py-2.5 pr-4"><Skeleton className="h-4 w-32" /><Skeleton className="mt-1 h-3 w-20" /></td>
                <td className="py-2.5 pr-4"><Skeleton className="h-4 w-40" /></td>
                <td className="py-2.5 pr-4"><Skeleton className="h-5 w-20 rounded-full" /></td>
                <td className="py-2.5 pr-4"><Skeleton className="h-4 w-24" /></td>
                <td className="py-2.5"><Skeleton className="h-4 w-24" /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (contacts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Users className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No contacts yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Add your first contact to start building relationships
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <Plus className="mr-1 h-4 w-4" />
            Create Contact
          </Button>
        )}
      </div>
    );
  }

  return (
    <div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            {headers.map((h) => (
              <th key={h.field} className={`pb-2 font-medium ${h.field !== 'created_at' ? 'pr-4' : ''}`}>
                <button
                  className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
                  onClick={() => toggleSort(h.field)}
                >
                  {h.label}
                  <SortIcon field={h.field} sortField={sortField} sortDir={sortDir} />
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((contact) => (
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
