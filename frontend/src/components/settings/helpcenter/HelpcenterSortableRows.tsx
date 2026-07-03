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
import { SOCIAL_PLATFORM_META, SocialPlatformIcon, type SocialPlatformMeta } from '@/components/docs/helpcenter/SocialPlatformIcon';

const SOCIAL_PLATFORM_OPTIONS: Array<{ value: HelpcenterSocialPlatform } & SocialPlatformMeta> = [
  { value: 'linkedin', ...SOCIAL_PLATFORM_META.linkedin },
  { value: 'x', ...SOCIAL_PLATFORM_META.x },
  { value: 'github', ...SOCIAL_PLATFORM_META.github },
  { value: 'youtube', ...SOCIAL_PLATFORM_META.youtube },
  { value: 'facebook', ...SOCIAL_PLATFORM_META.facebook },
  { value: 'instagram', ...SOCIAL_PLATFORM_META.instagram },
  { value: 'discord', ...SOCIAL_PLATFORM_META.discord },
  { value: 'slack', ...SOCIAL_PLATFORM_META.slack },
  { value: 'rss', ...SOCIAL_PLATFORM_META.rss },
  { value: 'website', ...SOCIAL_PLATFORM_META.website },
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

  const selectedMeta = SOCIAL_PLATFORM_META[link.platform || 'linkedin'] ?? SOCIAL_PLATFORM_META.website;
  const selectedPlatform = link.platform || 'linkedin';

  return (
    <div ref={setNodeRef} style={style} className="group flex items-center gap-2">
      <button type="button" {...attributes} {...listeners} className="shrink-0 cursor-grab touch-none text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing">
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <Select value={link.platform || 'linkedin'} onValueChange={(value) => onUpdate({ platform: value as HelpcenterSocialPlatform })}>
        <SelectTrigger className="h-8 w-[132px] shrink-0 text-xs">
          <SelectValue>
            <span className="flex min-w-0 items-center gap-2">
              <SocialPlatformIcon platform={selectedPlatform} meta={selectedMeta} className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <span className="truncate">{selectedMeta.label}</span>
            </span>
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {SOCIAL_PLATFORM_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              <span className="flex items-center gap-2">
                <SocialPlatformIcon platform={option.value} meta={option} className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span>{option.label}</span>
              </span>
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
