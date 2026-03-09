import { useMemo, useState } from 'react';
import { format } from 'date-fns';
import { ArrowUp, ArrowDown, ArrowUpDown, DollarSign, Plus } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import type { CRMDeal } from '@/lib/crmTypes';

interface DealsTableProps {
  deals: CRMDeal[];
  total: number;
  isLoading: boolean;
  onRowClick: (id: string) => void;
  onCreateClick?: () => void;
}

type SortField = 'name' | 'stage' | 'amount' | 'close_date' | 'created_at';
type SortDir = 'asc' | 'desc';

function SortIcon({ field, sortField, sortDir }: { field: SortField; sortField: SortField; sortDir: SortDir }) {
  if (sortField !== field) return <ArrowUpDown className="h-3 w-3 opacity-50" />;
  return sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />;
}

const headers: { field: SortField; label: string }[] = [
  { field: 'name', label: 'Deal' },
  { field: 'stage', label: 'Stage' },
  { field: 'amount', label: 'Amount' },
  { field: 'close_date', label: 'Close Date' },
  { field: 'created_at', label: 'Created' },
];

export function DealsTable({ deals, total, isLoading, onRowClick, onCreateClick }: DealsTableProps) {
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
    return [...deals].sort((a, b) => {
      let av: string | number;
      let bv: string | number;
      if (sortField === 'stage') {
        av = a.stage?.name ?? '';
        bv = b.stage?.name ?? '';
      } else if (sortField === 'amount') {
        av = a.amount ?? 0;
        bv = b.amount ?? 0;
      } else {
        av = (a[sortField] as string) ?? '';
        bv = (b[sortField] as string) ?? '';
      }
      const cmp = typeof av === 'number' ? av - (bv as number) : String(av).localeCompare(String(bv));
      return sortDir === 'asc' ? cmp : -cmp;
    });
  }, [deals, sortField, sortDir]);

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
                <td className="py-2.5 pr-4"><Skeleton className="h-5 w-20 rounded-full" /></td>
                <td className="py-2.5 pr-4"><Skeleton className="h-4 w-24" /></td>
                <td className="py-2.5 pr-4"><Skeleton className="h-4 w-24" /></td>
                <td className="py-2.5"><Skeleton className="h-4 w-24" /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (deals.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <DollarSign className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No deals yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Create your first deal to start tracking revenue
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <Plus className="mr-1 h-4 w-4" />
            Create Deal
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
          {sorted.map((deal) => (
            <tr
              key={deal.id}
              className="cursor-pointer border-b transition-colors hover:bg-muted/50"
              onClick={() => onRowClick(deal.id)}
            >
              <td className="py-2.5 pr-4">
                <div className="font-medium">{deal.name}</div>
                <div className="text-xs text-muted-foreground">{deal.display_id}</div>
              </td>
              <td className="py-2.5 pr-4">
                {deal.stage && <Badge variant="outline">{deal.stage.name}</Badge>}
              </td>
              <td className="py-2.5 pr-4">
                {deal.amount != null ? (
                  <span>{deal.currency} {deal.amount.toLocaleString()}</span>
                ) : (
                  <span className="text-muted-foreground">-</span>
                )}
              </td>
              <td className="py-2.5 pr-4 text-muted-foreground">
                {deal.close_date ? format(new Date(deal.close_date), 'MMM d, yyyy') : '-'}
              </td>
              <td className="py-2.5 text-muted-foreground">
                {format(new Date(deal.created_at), 'MMM d, yyyy')}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-2 text-xs text-muted-foreground">{total} total deals</p>
    </div>
  );
}
