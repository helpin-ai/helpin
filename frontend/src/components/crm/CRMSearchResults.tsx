import { useState } from 'react';
import { Search, Users, Building2, DollarSign } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useCRMSearch } from '@/hooks/queries/useCRM';

interface CRMSearchResultsProps {
  workspaceId: string;
  onSelectContact?: (id: string) => void;
  onSelectCompany?: (id: string) => void;
  onSelectDeal?: (id: string) => void;
}

export function CRMSearchResults({ workspaceId, onSelectContact, onSelectCompany, onSelectDeal }: CRMSearchResultsProps) {
  const [query, setQuery] = useState('');
  const { data, isLoading } = useCRMSearch(workspaceId, query);

  const results = data as { contacts?: Array<{ id: string; first_name: string; last_name?: string; email?: string }>; companies?: Array<{ id: string; name: string; domain?: string }>; deals?: Array<{ id: string; name: string; amount?: number; currency?: string }> } | undefined;

  return (
    <div className="space-y-4">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search contacts, companies, deals..."
          className="pl-9"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      {isLoading && query.length >= 2 && (
        <p className="text-sm text-muted-foreground">Searching...</p>
      )}

      {results && query.length >= 2 && (
        <div className="space-y-4">
          {results.contacts && results.contacts.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Users className="h-4 w-4" />
                  Contacts
                  <Badge variant="secondary" className="text-xs">{results.contacts.length}</Badge>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-1">
                {results.contacts.map((c) => (
                  <button
                    key={c.id}
                    className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                    onClick={() => onSelectContact?.(c.id)}
                  >
                    <span className="font-medium">{c.first_name} {c.last_name ?? ''}</span>
                    {c.email && <span className="text-xs text-muted-foreground">{c.email}</span>}
                  </button>
                ))}
              </CardContent>
            </Card>
          )}

          {results.companies && results.companies.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Building2 className="h-4 w-4" />
                  Companies
                  <Badge variant="secondary" className="text-xs">{results.companies.length}</Badge>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-1">
                {results.companies.map((c) => (
                  <button
                    key={c.id}
                    className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                    onClick={() => onSelectCompany?.(c.id)}
                  >
                    <span className="font-medium">{c.name}</span>
                    {c.domain && <span className="text-xs text-muted-foreground">{c.domain}</span>}
                  </button>
                ))}
              </CardContent>
            </Card>
          )}

          {results.deals && results.deals.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <DollarSign className="h-4 w-4" />
                  Deals
                  <Badge variant="secondary" className="text-xs">{results.deals.length}</Badge>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-1">
                {results.deals.map((d) => (
                  <button
                    key={d.id}
                    className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                    onClick={() => onSelectDeal?.(d.id)}
                  >
                    <span className="font-medium">{d.name}</span>
                    {d.amount != null && (
                      <span className="text-xs text-muted-foreground">
                        {d.currency ?? '$'} {d.amount.toLocaleString()}
                      </span>
                    )}
                  </button>
                ))}
              </CardContent>
            </Card>
          )}

          {(!results.contacts?.length && !results.companies?.length && !results.deals?.length) && (
            <p className="py-4 text-center text-sm text-muted-foreground">No results found for "{query}"</p>
          )}
        </div>
      )}
    </div>
  );
}
