import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useDockStore } from '@/stores/dockStore';

export function DockTranscriptViewPicker() {
  const view = useDockStore(state => state.transcriptView);
  const setView = useDockStore(state => state.setTranscriptView);
  return (
    <Select value={view} onValueChange={value => setView(value === 'detailed' ? 'detailed' : 'timeline')}>
      <SelectTrigger variant="ghost" size="sm" aria-label="Conversation view" className="h-8 shrink-0 gap-1 px-1.5 text-xs text-muted-foreground">
        <SelectValue />
      </SelectTrigger>
      <SelectContent data-helpin-dock-overlay align="end" className="z-[70]">
        <SelectItem value="timeline">Timeline</SelectItem>
        <SelectItem value="detailed">Detailed</SelectItem>
      </SelectContent>
    </Select>
  );
}
