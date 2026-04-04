import { BulbIcon, Tick01Icon, Cancel01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useSuggestions, useUpdateSuggestion } from '@/hooks/queries/useCRM';
import type { CRMSuggestion } from '@/lib/crmTypes';

interface SuggestionsPanelProps {
  workspaceId: string;
  objectType?: string;
  objectId?: string;
}

export function SuggestionsPanel({ workspaceId, objectType, objectId }: SuggestionsPanelProps) {
  const { data } = useSuggestions(workspaceId, {
    status: 'pending',
    ...(objectType && { suggestion_type: objectType }),
  });
  const updateSuggestion = useUpdateSuggestion(workspaceId);

  const suggestions = (data?.data ?? []) as CRMSuggestion[];
  const filtered = objectId
    ? suggestions.filter((s) => s.object_id === objectId)
    : suggestions;

  if (filtered.length === 0) return null;

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <BulbIcon className="h-4 w-4 text-yellow-500" />
          AI Suggestions
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {filtered.map((suggestion) => (
          <div key={suggestion.id} className="rounded-md border p-3">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-medium">{suggestion.title}</p>
                  <Badge variant="outline" className="text-xs">
                    {suggestion.suggestion_type.replace(/_/g, ' ')}
                  </Badge>
                </div>
                {suggestion.description && (
                  <p className="mt-1 text-xs text-muted-foreground">{suggestion.description}</p>
                )}
              </div>
              <div className="flex shrink-0 gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => updateSuggestion.mutate({ id: suggestion.id, status: 'accepted' })}
                  title="Accept"
                >
                  <Tick01Icon className="h-3.5 w-3.5 text-green-500" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => updateSuggestion.mutate({ id: suggestion.id, status: 'dismissed' })}
                  title="Dismiss"
                >
                  <Cancel01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                </Button>
              </div>
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
