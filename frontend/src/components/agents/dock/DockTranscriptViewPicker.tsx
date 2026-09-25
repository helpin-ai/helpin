import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useDockStore } from '@/stores/dockStore';

export function DockTranscriptViewPicker({ label = 'Conversation view', menuClassName = 'z-[70]', dockOverlay = true }: { label?: string; menuClassName?: string; dockOverlay?: boolean } = {}) {
  const view = useDockStore(state => state.transcriptView);
  const setView = useDockStore(state => state.setTranscriptView);
  return (
    <Select value={view} onValueChange={value => setView(value === 'detailed' ? 'detailed' : 'timeline')}>
      <SelectTrigger variant="ghost" size="sm" aria-label={label} className="h-8 shrink-0 gap-1 px-1.5 text-xs text-muted-foreground">
        <SelectValue />
      </SelectTrigger>
      <SelectContent data-helpin-dock-overlay={dockOverlay || undefined} align="end" className={menuClassName}>
        <SelectItem value="timeline">Timeline</SelectItem>
        <SelectItem value="detailed">Detailed</SelectItem>
      </SelectContent>
    </Select>
  );
}
