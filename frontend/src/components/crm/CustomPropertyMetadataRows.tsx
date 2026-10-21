import { Tag01Icon } from '@/lib/icons';
import { customPropertyRows } from '@/lib/crmCustomProperties';

/**
 * Read-only custom property rows for a detail rail's three-column
 * icon / label / value grid, matching the rail's native field rows.
 */
export function CustomPropertyMetadataRows({ properties }: { properties: Record<string, unknown> | null | undefined }) {
  return customPropertyRows(properties).map((row) => (
    <div key={row.key} className="contents" data-custom-property={row.key}>
      <Tag01Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 self-start text-muted-foreground" />
      <span className="mt-0.5 self-start truncate text-[12px] text-muted-foreground" title={row.label}>{row.label}</span>
      <div className="min-w-0 whitespace-pre-wrap break-words px-1.5 py-0.5 text-xs text-foreground">
        {row.href ? (
          <a href={row.href} target="_blank" rel="noopener noreferrer" className="break-all text-quiet-accent hover:text-quiet-accent-hover hover:underline">
            {row.value}
          </a>
        ) : row.value}
      </div>
    </div>
  ));
}
