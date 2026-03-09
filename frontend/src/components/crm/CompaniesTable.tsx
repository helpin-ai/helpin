import { format } from 'date-fns';
import type { CRMCompany } from '@/lib/crmTypes';

interface CompaniesTableProps {
  companies: CRMCompany[];
  total: number;
  isLoading: boolean;
  onRowClick: (id: string) => void;
}

export function CompaniesTable({ companies, total, isLoading, onRowClick }: CompaniesTableProps) {
  if (isLoading) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Loading companies...</div>;
  }

  if (companies.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center">
        <p className="text-muted-foreground">No companies found</p>
        <p className="mt-1 text-sm text-muted-foreground/70">Create your first company to get started</p>
      </div>
    );
  }

  return (
    <div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="pb-2 pr-4 font-medium">Company</th>
            <th className="pb-2 pr-4 font-medium">Domain</th>
            <th className="pb-2 pr-4 font-medium">Industry</th>
            <th className="pb-2 pr-4 font-medium">Employees</th>
            <th className="pb-2 font-medium">Created</th>
          </tr>
        </thead>
        <tbody>
          {companies.map((company) => (
            <tr
              key={company.id}
              className="cursor-pointer border-b transition-colors hover:bg-muted/50"
              onClick={() => onRowClick(company.id)}
            >
              <td className="py-2.5 pr-4">
                <div className="font-medium">{company.name}</div>
                <div className="text-xs text-muted-foreground">{company.display_id}</div>
              </td>
              <td className="py-2.5 pr-4 text-muted-foreground">{company.domain ?? '-'}</td>
              <td className="py-2.5 pr-4 text-muted-foreground">{company.industry ?? '-'}</td>
              <td className="py-2.5 pr-4 text-muted-foreground">{company.employee_count ?? '-'}</td>
              <td className="py-2.5 text-muted-foreground">
                {format(new Date(company.created_at), 'MMM d, yyyy')}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-2 text-xs text-muted-foreground">{total} total companies</p>
    </div>
  );
}
