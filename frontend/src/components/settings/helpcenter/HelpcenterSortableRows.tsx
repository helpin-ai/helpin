import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { DragDropVerticalIcon, Delete01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type {
  HelpcenterFooterLink,
  HelpcenterHeaderLink,
  HelpcenterHeaderLinkStyle,
  HelpcenterSocialLink,
  HelpcenterSocialPlatform,
} from '@/lib/docsTypes';

const SOCIAL_PLATFORM_OPTIONS: Array<{ value: HelpcenterSocialPlatform; label: string }> = [
  { value: 'linkedin', label: 'LinkedIn' },
  { value: 'x', label: 'X' },
  { value: 'github', label: 'GitHub' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'facebook', label: 'Facebook' },
  { value: 'instagram', label: 'Instagram' },
  { value: 'discord', label: 'Discord' },
  { value: 'slack', label: 'Slack' },
  { value: 'rss', label: 'RSS' },
  { value: 'website', label: 'Website' },
];

export function SortableHeaderLinkRow({
  id,
  link,
  onUpdate,
  onRemove,
}: {
  id: string;
  link: HelpcenterHeaderLink;
  onUpdate: (patch: Partial<HelpcenterHeaderLink>) => void;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div ref={setNodeRef} style={style} className="group flex items-center gap-2">
      <button type="button" {...attributes} {...listeners} className="shrink-0 cursor-grab touch-none text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing">
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <Input
        value={link.label}
        onChange={(event) => onUpdate({ label: event.target.value })}
        placeholder="Label"
        className="h-8 w-28 text-sm"
      />
      <Input
        value={link.url}
        onChange={(event) => onUpdate({ url: event.target.value })}
        placeholder="https://..."
        className="h-8 flex-1 text-sm"
      />
      <Select value={link.style || 'text'} onValueChange={(value) => onUpdate({ style: value as HelpcenterHeaderLinkStyle })}>
        <SelectTrigger className="h-8 w-[90px] shrink-0 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="text">Text</SelectItem>
          <SelectItem value="button">Button</SelectItem>
        </SelectContent>
      </Select>
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" onClick={onRemove}>
        <Delete01Icon className="h-3.5 w-3.5 text-destructive" />
      </Button>
    </div>
  );
}

export function SortableFooterLinkRow({
  id,
  link,
  onUpdate,
  onRemove,
}: {
  id: number;
  link: HelpcenterFooterLink;
  onUpdate: (patch: Partial<HelpcenterFooterLink>) => void;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div ref={setNodeRef} style={style} className="group flex items-center gap-2">
      <button type="button" {...attributes} {...listeners} className="shrink-0 cursor-grab touch-none text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing">
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <Input
        value={link.label}
        onChange={(event) => onUpdate({ label: event.target.value })}
        placeholder="Label"
        className="h-8 w-28 text-sm"
      />
      <Input
        value={link.url}
        onChange={(event) => onUpdate({ url: event.target.value })}
        placeholder="https://..."
        className="h-8 flex-1 text-sm"
      />
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" onClick={onRemove}>
        <Delete01Icon className="h-3.5 w-3.5 text-destructive" />
      </Button>
    </div>
  );
}

export function SortableSocialLinkRow({
  id,
  link,
  onUpdate,
  onRemove,
}: {
  id: number;
  link: HelpcenterSocialLink;
  onUpdate: (patch: Partial<HelpcenterSocialLink>) => void;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div ref={setNodeRef} style={style} className="group flex items-center gap-2">
      <button type="button" {...attributes} {...listeners} className="shrink-0 cursor-grab touch-none text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing">
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <Select value={link.platform || 'linkedin'} onValueChange={(value) => onUpdate({ platform: value as HelpcenterSocialPlatform })}>
        <SelectTrigger className="h-8 w-[118px] shrink-0 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SOCIAL_PLATFORM_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        value={link.url}
        onChange={(event) => onUpdate({ url: event.target.value })}
        placeholder="https://..."
        className="h-8 flex-1 text-sm"
      />
      <Button type="button" variant="ghost" size="icon" className="h-8 w-8 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" onClick={onRemove}>
        <Delete01Icon className="h-3.5 w-3.5 text-destructive" />
      </Button>
    </div>
  );
}
