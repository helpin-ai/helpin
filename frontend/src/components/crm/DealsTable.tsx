import { format } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import type { CRMDeal } from '@/lib/crmTypes';

interface DealsTableProps {
  deals: CRMDeal[];
  total: number;
  isLoading: boolean;
  onRowClick: (id: string) => void;
}

export function DealsTable({ deals, total, isLoading, onRowClick }: DealsTableProps) {
  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading deals...</div>;
  }

  if (deals.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center">
        <p className="text-muted-foreground">No deals found</p>
        <p className="mt-1 text-sm text-muted-foreground/70">Create your first deal to get started</p>
      </div>
    );
  }

  return (
    <div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-2 pr-4 font-medium">Deal</th>
            <th className="pb-2 pr-4 font-medium">Stage</th>
            <th className="pb-2 pr-4 font-medium">Amount</th>
            <th className="pb-2 pr-4 font-medium">Close Date</th>
            <th className="pb-2 font-medium">Created</th>
          </tr>
        </thead>
        <tbody>
          {deals.map((deal) => (
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
