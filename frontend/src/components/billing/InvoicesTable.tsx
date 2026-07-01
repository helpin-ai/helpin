import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { formatCents, formatDate } from '@/lib/billingUtils';
import type { Invoice } from '@/lib/billingTypes';

export function InvoicesTable({ invoices }: { invoices: Invoice[] }) {
  if (invoices.length === 0) {
    return (
      <div className="rounded-xl border bg-card p-8 text-center text-sm text-muted-foreground">
        No invoices yet.
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Invoice</TableHead>
            <TableHead>Date</TableHead>
            <TableHead>Workspace</TableHead>
            <TableHead className="text-right">Amount</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="text-right">Links</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {invoices.map((inv) => (
            <TableRow key={inv.id}>
              <TableCell className="font-medium">{inv.number || inv.id}</TableCell>
              <TableCell className="text-muted-foreground">{formatDate(inv.created)}</TableCell>
              <TableCell className="text-muted-foreground">{inv.workspace_name}</TableCell>
              <TableCell className="text-right tabular-nums">
                {formatCents(inv.amount_cents)}
              </TableCell>
              <TableCell>
                <span className="capitalize text-muted-foreground">{inv.status}</span>
              </TableCell>
              <TableCell className="text-right">
                <div className="flex justify-end gap-1">
                  {inv.hosted_url && (
                    <Button asChild variant="ghost" size="sm">
                      <a href={inv.hosted_url} target="_blank" rel="noopener noreferrer">
                        View
                      </a>
                    </Button>
                  )}
                  {inv.pdf_url && (
                    <Button asChild variant="ghost" size="sm">
                      <a href={inv.pdf_url} target="_blank" rel="noopener noreferrer">
                        PDF
                      </a>
                    </Button>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
