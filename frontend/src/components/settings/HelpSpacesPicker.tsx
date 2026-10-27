import { Switch } from '@/components/ui/switch';
import type { DocsSpace } from '@/lib/docsTypes';

/**
 * HelpSpacesPicker chooses the help center spaces customers are offered. The
 * Messenger's Help tab and the customer portal's article suggestions share
 * this one selection (`widget_help_space_ids`), so it appears on both settings
 * pages and changing it on either changes both.
 */
export function HelpSpacesPicker({ spaces, loading, selectedIds, onToggle, disabled }: {
  spaces: DocsSpace[];
  loading: boolean;
  selectedIds: string[];
  onToggle: (spaceId: string, enabled: boolean) => void;
  disabled?: boolean;
}) {
  const external = spaces.filter((space) => space.type === 'external_capable');
  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">Shared by the chat widget's Help tab and the customer portal's article suggestions. Changing it here changes both.</p>
      {loading ? (
        <p className="text-sm text-muted-foreground">Loading available spaces...</p>
      ) : external.length === 0 ? (
        <p className="text-sm text-muted-foreground">Create an external space in Docs to offer articles to customers.</p>
      ) : (
        external.map((space) => (
          <div key={space.id} className="flex items-center justify-between gap-4 rounded-lg border p-4">
            <div className="min-w-0">
              <p className="text-sm font-medium">{space.name}</p>
              <p className="text-xs text-muted-foreground">/{space.slug}</p>
            </div>
            <Switch
              checked={selectedIds.includes(space.id)}
              disabled={disabled}
              onCheckedChange={(checked) => onToggle(space.id, checked)}
              aria-label={`Offer ${space.name} to customers`}
            />
          </div>
        ))
      )}
    </div>
  );
}
