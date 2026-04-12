import { useEffect, useRef, useState, useCallback, type ChangeEvent, type FormEvent } from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  arrayMove,
  useSortable,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { Checkbox } from '@/components/ui/checkbox';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { useDocsHelpcenterLocales, useUpdateDocsHelpcenterLocales } from '@/hooks/queries';
import { HelpcenterLocalesCard } from '@/components/settings/helpcenter/HelpcenterLocalesCard';
import { HelpcenterTranslationsTable } from '@/components/settings/helpcenter/HelpcenterTranslationsTable';
import { SortableFooterLinkRow, SortableHeaderLinkRow } from '@/components/settings/helpcenter/HelpcenterSortableRows';
import {
  PlusSignIcon, InformationCircleIcon, ArrowDown01Icon, Cancel01Icon,
  GlobeIcon, PaintBoardIcon, LayoutGridIcon, Link01Icon, Image01Icon,
  LanguageCircleIcon, DragDropVerticalIcon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import { IconPicker, StoredIcon } from '@/components/ui/icon-picker';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { buildCollectionTreeOptions } from '@/components/docs/CollectionTreePicker';
import type {
  HelpcenterHeaderLink,
  HelpcenterFooterLink,
  HelpcenterThemeMode,
  HomepageFeaturedCard,
  DocsHelpcenterLocalesConfig,
  DocsSpace,
  DocsCollection,
} from '@/lib/docsTypes';

type HeaderLinkWithId = HelpcenterHeaderLink & { _id: string };

let _linkIdCounter = 0;
function nextLinkId() { return `link-${++_linkIdCounter}-${Date.now()}`; }
function withIds(links: HelpcenterHeaderLink[]): HeaderLinkWithId[] {
  return links.map(l => ({ ...l, _id: nextLinkId() }));
}

function findCollectionByCardLinkValue(
  collections: DocsCollection[],
  linkValue: string,
): DocsCollection | undefined {
  return collections.find((collection) =>
    collection.id === linkValue || collection.slug === linkValue,
  );
}

function findFeaturedCardForCollection(
  cards: HomepageFeaturedCard[],
  collection: DocsCollection,
  spaceSlug: string,
): HomepageFeaturedCard | undefined {
  return cards.find((card) =>
    card.link_type === 'collection' &&
    card.space_slug === spaceSlug &&
    (card.link_value === collection.id || card.link_value === collection.slug),
  );
}

function syncFeaturedCardsForCollections(
  cards: HomepageFeaturedCard[],
  collections: DocsCollection[],
  spaceSlug: string,
): HomepageFeaturedCard[] {
  return cards.map((card) => {
    if (card.link_type !== 'collection' || card.space_slug !== spaceSlug) {
      return card;
    }
    const collection = findCollectionByCardLinkValue(collections, card.link_value);
    if (!collection) {
      return card;
    }
    return {
      ...card,
      title: collection.name,
      icon: collection.icon ?? '',
      link_value: collection.slug,
      space_slug: spaceSlug,
    };
  });
}

function orderCollectionsForFeaturedCards(
  collections: DocsCollection[],
  cards: HomepageFeaturedCard[],
  spaceSlug: string,
): DocsCollection[] {
  const orderedSelectedCollections = cards
    .filter((card) => card.link_type === 'collection' && card.space_slug === spaceSlug)
    .map((card) => findCollectionByCardLinkValue(collections, card.link_value))
    .filter((collection): collection is DocsCollection => !!collection);
  const selectedIds = new Set(orderedSelectedCollections.map((collection) => collection.id));
  const remainingCollections = collections.filter((collection) => !selectedIds.has(collection.id));
  return [...orderedSelectedCollections, ...remainingCollections];
}

function reorderFeaturedCardsForSpace(
  cards: HomepageFeaturedCard[],
  collections: DocsCollection[],
  spaceSlug: string,
  reorderedCollectionIds: string[],
): HomepageFeaturedCard[] {
  const selectedCards = cards.filter(
    (card) => card.link_type === 'collection' && card.space_slug === spaceSlug,
  );
  const selectedByCollectionId = new Map(
    selectedCards.map((card) => {
      const collection = findCollectionByCardLinkValue(collections, card.link_value);
      return [collection?.id ?? card.link_value, card] as const;
    }),
  );
  const reorderedSelectedCards = reorderedCollectionIds
    .map((id) => selectedByCollectionId.get(id))
    .filter((card): card is HomepageFeaturedCard => !!card);

  if (reorderedSelectedCards.length !== selectedCards.length) {
    return cards;
  }

  let selectedIndex = 0;
  return cards.map((card) => {
    if (card.link_type !== 'collection' || card.space_slug !== spaceSlug) {
      return card;
    }
    const nextCard = reorderedSelectedCards[selectedIndex];
    selectedIndex += 1;
    return nextCard;
  });
}

function SortableFeaturedCollectionRow({
  id,
  collection,
  card,
  pathLabel,
  onToggle,
  onDescriptionChange,
  onIconChange,
}: {
  id: string;
  collection: DocsCollection;
  card: HomepageFeaturedCard;
  /**
   * Optional breadcrumb-style ancestor path, e.g. "Parent / Middle".
   * When present, rendered as a small muted line above the collection
   * name so users can disambiguate nested collections that share a
   * leaf name across different parents.
   */
  pathLabel?: string;
  onToggle: () => void;
  onDescriptionChange: (value: string) => void;
  onIconChange: (value: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-3 rounded-lg border border-primary/20 bg-primary/[0.03] p-3"
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="shrink-0 cursor-grab touch-none text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
        aria-label={`Reorder ${collection.name}`}
      >
        <DragDropVerticalIcon className="h-4 w-4" />
      </button>
      <Checkbox
        checked
        onCheckedChange={onToggle}
      />
      <div className="flex-1 grid gap-2 grid-cols-[40px_140px_1fr] items-center">
        <IconPicker
          value={card.icon}
          onChange={onIconChange}
        />
        <div className="min-w-0">
          {pathLabel && (
            <div className="text-[10px] text-muted-foreground/70 truncate">{pathLabel}</div>
          )}
          <span className="text-sm font-medium truncate block">{collection.name}</span>
        </div>
        <Input
          value={card.description}
          onChange={(event) => onDescriptionChange(event.target.value)}
          placeholder="Short description that goes on the featured card"
          className="h-8 text-sm"
        />
      </div>
    </div>
  );
}

interface ConfigState {
  subdomain: string;
  custom_domain: string;
  brand_name: string;
  brand_logo_url: string;
  brand_logo_dark_url: string;
  brand_color: string;
  favicon_url: string;
  theme_mode: HelpcenterThemeMode;
  header_links: HeaderLinkWithId[];
  footer_copyright_text: string;
  footer_links: HelpcenterFooterLink[];
  homepage_hero_title: string;
  homepage_hero_subtitle: string;
  homepage_featured_cards: HomepageFeaturedCard[];
  search_placeholder: string;
  protected_terms: string[];
  is_published: boolean;
  seo_title: string;
  seo_description: string;
  support_email: string;
}

const DEFAULT_CONFIG: ConfigState = {
  subdomain: '',
  custom_domain: '',
  brand_name: '',
  brand_logo_url: '',
  brand_logo_dark_url: '',
  brand_color: '#3b82f6',
  favicon_url: '',
  theme_mode: 'system',
  header_links: [],
  footer_copyright_text: '',
  footer_links: [],
  homepage_hero_title: '',
  homepage_hero_subtitle: '',
  homepage_featured_cards: [],
  search_placeholder: '',
  protected_terms: [],
  is_published: false,
  seo_title: '',
  seo_description: '',
  support_email: '',
};

const DEFAULT_LOCALES_CONFIG: DocsHelpcenterLocalesConfig = {
  default_locale: 'en',
  enabled_locales: ['en'],
  show_language_switcher: true,
  fallback_to_default_locale: true,
};

// Derive a URL-safe slug from a brand name.
function slugifyBrand(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

// Generate smart defaults from a brand name (only fills empty fields).
function deriveDefaults(brandName: string, current: ConfigState): Partial<ConfigState> {
  const name = brandName.trim();
  if (!name) return {};
  const year = new Date().getFullYear();
  const defaults: Partial<ConfigState> = {};
  if (!current.subdomain) defaults.subdomain = slugifyBrand(name);
  if (!current.seo_title) defaults.seo_title = `${name} Help Center`;
  if (!current.seo_description) defaults.seo_description = `Find answers, guides, and documentation for ${name}.`;
  if (!current.homepage_hero_title) defaults.homepage_hero_title = 'How can we help?';
  if (!current.homepage_hero_subtitle) defaults.homepage_hero_subtitle = 'Search our knowledge base or browse topics below';
  if (!current.search_placeholder) defaults.search_placeholder = 'Search for articles...';
  if (!current.footer_copyright_text) defaults.footer_copyright_text = `\u00A9 ${year} ${name}. All rights reserved.`;
  return defaults;
}

export function HelpcenterTab({ workspaceId, workspaceName }: { workspaceId: string; workspaceName: string }) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<ConfigState>(DEFAULT_CONFIG);
  const [spaces, setSpaces] = useState<DocsSpace[]>([]);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());
  const toggleSection = (key: string) => {
    setExpandedSections(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  };
  const isExpanded = (key: string) => expandedSections.has(key);
  const [homepageSpaceSlug, setHomepageSpaceSlug] = useState('');
  const [spaceCollections, setSpaceCollections] = useState<DocsCollection[]>([]);
  const { data: localesConfig } = useDocsHelpcenterLocales(workspaceId);
  const updateLocales = useUpdateDocsHelpcenterLocales(workspaceId);
  const normalizedLocalesConfig: DocsHelpcenterLocalesConfig = localesConfig
    ? {
        ...localesConfig,
        default_locale: localesConfig.default_locale || DEFAULT_LOCALES_CONFIG.default_locale,
        enabled_locales:
          Array.isArray(localesConfig.enabled_locales) && localesConfig.enabled_locales.length > 0
            ? localesConfig.enabled_locales
            : [localesConfig.default_locale || DEFAULT_LOCALES_CONFIG.default_locale],
        show_language_switcher: localesConfig.show_language_switcher !== false,
        fallback_to_default_locale: localesConfig.fallback_to_default_locale !== false,
      }
    : DEFAULT_LOCALES_CONFIG;

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      const { docsService } = await import('@/lib/services/docsService');
      const res = await docsService.getHelpcenterConfig(workspaceId);
      if (res.data) {
        const d = res.data;
        const loaded: ConfigState = {
          subdomain: d.subdomain ?? '',
          custom_domain: d.custom_domain ?? '',
          brand_name: d.brand_name ?? '',
          brand_logo_url: d.brand_logo_url ?? '',
          brand_logo_dark_url: d.brand_logo_dark_url ?? '',
          brand_color: d.brand_color ?? '#3b82f6',
          favicon_url: d.favicon_url ?? '',
          theme_mode: d.theme_mode ?? 'system',
          header_links: withIds(d.header_links ?? []),
          footer_copyright_text: d.footer_config?.copyright_text ?? '',
          footer_links: d.footer_config?.links ?? [],
          homepage_hero_title: d.homepage_config?.hero_title ?? '',
          homepage_hero_subtitle: d.homepage_config?.hero_subtitle ?? '',
          homepage_featured_cards: d.homepage_config?.featured_cards ?? [],
          search_placeholder: d.search_placeholder ?? '',
          protected_terms: d.protected_terms ?? [],
          is_published: d.is_published ?? false,
          seo_title: d.seo_title ?? '',
          seo_description: d.seo_description ?? '',
          support_email: d.support_email ?? '',
        };
        // Auto-fill brand name from workspace name if not yet set, then derive defaults
        const effectiveBrand = loaded.brand_name || workspaceName;
        if (!loaded.brand_name && workspaceName) loaded.brand_name = workspaceName;
        const defaults = deriveDefaults(effectiveBrand, loaded);
        setConfig({ ...loaded, ...defaults });
      } else {
        // No config exists yet — pre-fill everything from workspace name
        const fresh = { ...DEFAULT_CONFIG, brand_name: workspaceName };
        setConfig({ ...fresh, ...deriveDefaults(workspaceName, fresh) });
      }
      // Load external_capable spaces for featured card pickers
      const spacesRes = await docsService.listSpaces(workspaceId);
      const extSpaces = spacesRes.data?.filter(s => s.type === 'external_capable') ?? [];
      setSpaces(extSpaces);

      // If existing cards reference a space, auto-select it and sync icons from collections
      const existingSlug = (res.data?.homepage_config?.featured_cards ?? []).find(
        (card) => card.link_type === 'collection' && card.space_slug,
      )?.space_slug;
      if (existingSlug) {
        const space = extSpaces.find(s => s.slug === existingSlug);
        if (space) {
          setHomepageSpaceSlug(existingSlug);
          const colRes = await docsService.listCollections(workspaceId, space.id);
          if (colRes.data) {
            setSpaceCollections(colRes.data);
            const existingCards = res.data?.homepage_config?.featured_cards ?? [];
            const synced = syncFeaturedCardsForCollections(
              existingCards,
              colRes.data,
              space.slug,
            );
            setConfig(prev => ({ ...prev, homepage_featured_cards: synced }));
          }
        }
      }

      setLoading(false);
    };
    load();
  }, [workspaceId, workspaceName]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.updateHelpcenterConfig(workspaceId, {
      subdomain: config.subdomain || undefined,
      custom_domain: config.custom_domain || undefined,
      brand_name: config.brand_name || undefined,
      brand_logo_url: config.brand_logo_url || undefined,
      brand_logo_dark_url: config.brand_logo_dark_url || undefined,
      brand_color: config.brand_color || undefined,
      favicon_url: config.favicon_url || undefined,
      theme_mode: config.theme_mode,
      header_links: config.header_links,
      footer_config: {
        copyright_text: config.footer_copyright_text,
        links: config.footer_links,
      },
      homepage_config: {
        hero_title: config.homepage_hero_title,
        hero_subtitle: config.homepage_hero_subtitle,
        featured_cards: config.homepage_featured_cards,
      },
      search_placeholder: config.search_placeholder || undefined,
      protected_terms: config.protected_terms.filter(Boolean),
      is_published: config.is_published,
      seo_title: config.seo_title || undefined,
      seo_description: config.seo_description || undefined,
      support_email: config.support_email || undefined,
    });
    // Sync icon changes back to collections
    for (const card of config.homepage_featured_cards) {
      if (card.link_type !== 'collection' || !card.link_value) continue;
      const col = findCollectionByCardLinkValue(spaceCollections, card.link_value);
      if (col && (col.icon ?? '') !== card.icon) {
        await docsService.updateCollection(workspaceId, col.id, { icon: card.icon });
      }
    }

    setSaving(false);
    if (res.error) {
      toast.error(res.error);
    } else {
      toast.success('Help center settings saved');
    }
  };

  // ── Header link helpers ──
  const addHeaderLink = () => {
    setConfig({
      ...config,
      header_links: [...config.header_links, { _id: nextLinkId(), label: '', url: '', external: true, style: 'text' as const, position: config.header_links.length }],
    });
  };

  const updateHeaderLink = (index: number, patch: Partial<HelpcenterHeaderLink>) => {
    const links = [...config.header_links];
    links[index] = { ...links[index], ...patch };
    setConfig({ ...config, header_links: links });
  };

  const removeHeaderLink = (index: number) => {
    const filtered = config.header_links.filter((_, i) => i !== index);
    setConfig({ ...config, header_links: filtered.map((l, i) => ({ ...l, position: i })) });
  };

  // ── Header link drag-and-drop ──
  const headerLinkIds = config.header_links.map(l => l._id);
  const dndSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const handleHeaderDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    setConfig(prev => {
      const ids = prev.header_links.map(l => l._id);
      const oldIndex = ids.indexOf(active.id as string);
      const newIndex = ids.indexOf(over.id as string);
      const reordered = arrayMove(prev.header_links, oldIndex, newIndex);
      return { ...prev, header_links: reordered.map((l, i) => ({ ...l, position: i })) };
    });
  }, []);

  // ── Footer link helpers ──
  const addFooterLink = () => {
    setConfig({
      ...config,
      footer_links: [...config.footer_links, { label: '', url: '' }],
    });
  };

  const updateFooterLink = (index: number, patch: Partial<HelpcenterFooterLink>) => {
    const links = [...config.footer_links];
    links[index] = { ...links[index], ...patch };
    setConfig({ ...config, footer_links: links });
  };

  const removeFooterLink = (index: number) => {
    setConfig({ ...config, footer_links: config.footer_links.filter((_, i) => i !== index) });
  };

  const handleFooterDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    setConfig(prev => {
      const oldIndex = Number(active.id);
      const newIndex = Number(over.id);
      return { ...prev, footer_links: arrayMove(prev.footer_links, oldIndex, newIndex) };
    });
  }, []);

  // ── Featured card helpers (space-driven) ──
  const handleHomepageSpaceChange = async (slug: string) => {
    setHomepageSpaceSlug(slug);
    const space = spaces.find(s => s.slug === slug);
    if (!space) return;
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.listCollections(workspaceId, space.id);
    const cols = res.data ?? [];
    setSpaceCollections(cols);
    setConfig((prev) => {
      const existingCardsForSpace = prev.homepage_featured_cards.filter(
        (card) => card.link_type === 'collection' && card.space_slug === slug,
      );
      if (existingCardsForSpace.length > 0) {
        return {
          ...prev,
          homepage_featured_cards: syncFeaturedCardsForCollections(
            prev.homepage_featured_cards,
            cols,
            slug,
          ),
        };
      }

      const cards: HomepageFeaturedCard[] = cols.map(col => ({
        title: col.name,
        description: col.description ?? '',
        icon: col.icon ?? '',
        link_type: 'collection',
        link_value: col.slug,
        space_slug: slug,
      }));
      return { ...prev, homepage_featured_cards: cards };
    });
  };

  const toggleCollection = (colId: string) => {
    const col = spaceCollections.find(c => c.id === colId);
    if (!col) return;
    const exists = config.homepage_featured_cards.find(c => c.link_value === col.slug);
    if (exists) {
      setConfig(prev => ({
        ...prev,
        homepage_featured_cards: prev.homepage_featured_cards.filter(c => c.link_value !== col.slug),
      }));
    } else {
      // Insert the new card at the position matching the collection's
      // natural order in the space, not at the end. This way unchecking
      // and re-checking a collection puts it back where it was instead
      // of dumping it at the bottom of the featured list.
      const newCard: HomepageFeaturedCard = {
        title: col.name,
        description: col.description ?? '',
        icon: col.icon ?? '',
        link_type: 'collection',
        link_value: col.slug,
        space_slug: homepageSpaceSlug,
      };
      setConfig(prev => {
        const spaceCards = prev.homepage_featured_cards.filter(
          c => c.link_type === 'collection' && c.space_slug === homepageSpaceSlug,
        );
        const otherCards = prev.homepage_featured_cards.filter(
          c => !(c.link_type === 'collection' && c.space_slug === homepageSpaceSlug),
        );
        // Find where this collection sits relative to existing cards
        // based on the collections' natural order in the space.
        const collectionSlugs = spaceCollections.map(c => c.slug);
        const newIdx = collectionSlugs.indexOf(col.slug);
        let insertAt = spaceCards.length;
        for (let i = 0; i < spaceCards.length; i++) {
          const cardIdx = collectionSlugs.indexOf(spaceCards[i].link_value);
          if (cardIdx > newIdx) {
            insertAt = i;
            break;
          }
        }
        const updatedSpaceCards = [...spaceCards];
        updatedSpaceCards.splice(insertAt, 0, newCard);
        return {
          ...prev,
          homepage_featured_cards: [...otherCards, ...updatedSpaceCards],
        };
      });
    }
  };

  const updateCardByCollectionId = (colId: string, patch: Partial<HomepageFeaturedCard>) => {
    const col = spaceCollections.find(c => c.id === colId);
    if (!col) return;
    setConfig(prev => ({
      ...prev,
      homepage_featured_cards: prev.homepage_featured_cards.map(c =>
        c.link_value === col.slug ? { ...c, ...patch } : c
      ),
    }));
  };

  const handleFeaturedCardsDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id || !homepageSpaceSlug) return;

    const orderedCollections = orderCollectionsForFeaturedCards(
      spaceCollections,
      config.homepage_featured_cards,
      homepageSpaceSlug,
    );
    const selectedIds = orderedCollections
      .filter((collection) =>
        !!findFeaturedCardForCollection(
          config.homepage_featured_cards,
          collection,
          homepageSpaceSlug,
        ),
      )
      .map((collection) => collection.id);
    const oldIndex = selectedIds.indexOf(active.id as string);
    const newIndex = selectedIds.indexOf(over.id as string);
    if (oldIndex === -1 || newIndex === -1) return;

    const reorderedIds = arrayMove(selectedIds, oldIndex, newIndex);
    setConfig((prev) => ({
      ...prev,
      homepage_featured_cards: reorderFeaturedCardsForSpace(
        prev.homepage_featured_cards,
        spaceCollections,
        homepageSpaceSlug,
        reorderedIds,
      ),
    }));
  }, [config.homepage_featured_cards, homepageSpaceSlug, spaceCollections]);

  // ── Asset upload helpers ──
  const logoInputRef = useRef<HTMLInputElement>(null);
  const logoDarkInputRef = useRef<HTMLInputElement>(null);
  const faviconInputRef = useRef<HTMLInputElement>(null);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const [uploadingLogoDark, setUploadingLogoDark] = useState(false);
  const [uploadingFavicon, setUploadingFavicon] = useState(false);

  const handleAssetUpload = async (
    e: ChangeEvent<HTMLInputElement>,
    assetType: 'logo' | 'logo_dark' | 'favicon',
    setUploading: (v: boolean) => void,
    field: 'brand_logo_url' | 'brand_logo_dark_url' | 'favicon_url',
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file');
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error('Image must be under 2 MB');
      return;
    }
    setUploading(true);
    const { docsService } = await import('@/lib/services/docsService');
    const res = await docsService.uploadHelpcenterAsset(workspaceId, assetType, file);
    setUploading(false);
    e.target.value = '';
    if (res.error || !res.data) {
      toast.error(res.error ?? 'Upload failed');
      return;
    }
    setConfig((prev) => ({ ...prev, [field]: res.data!.url }));
    toast.success(`${assetType === 'favicon' ? 'Favicon' : 'Logo'} uploaded`);
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-16 w-full rounded-lg" />
        <Skeleton className="h-48 w-full rounded-lg" />
        <Skeleton className="h-32 w-full rounded-lg" />
      </div>
    );
  }

  // Featured cards are homepage-level tiles — only top-level
  // collections (no parent) make sense here. Sub-collections are
  // reachable from their parent collection's page.
  const topLevelCollections = spaceCollections.filter(
    (c) => !c.parent_collection_id,
  );
  const orderedSpaceCollections = orderCollectionsForFeaturedCards(
    topLevelCollections,
    config.homepage_featured_cards,
    homepageSpaceSlug,
  );
  const selectedFeaturedCollectionIds = orderedSpaceCollections
    .filter((collection) =>
      !!findFeaturedCardForCollection(
        config.homepage_featured_cards,
        collection,
        homepageSpaceSlug,
      ),
    )
    .map((collection) => collection.id);

  return (
    <form onSubmit={handleSave} className="space-y-5">
      {/* ── Top Actions ── */}
      <div className="flex items-center justify-end">
        <Button type="submit" disabled={saving} size="sm">
          {saving ? 'Saving...' : 'Save Changes'}
        </Button>
      </div>

      {/* ── Publish Status Bar ── */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-card p-4">
        <div>
          <p className="text-sm font-medium">Help Center</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            {config.is_published
              ? 'Your help center is publicly accessible.'
              : 'Toggle to make your help center visible to the public.'}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge
            variant={config.is_published ? 'default' : 'secondary'}
            className={`text-[11px] px-1.5 py-0 ${config.is_published ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 hover:bg-emerald-500/15' : ''}`}
          >
            {config.is_published ? 'Live' : 'Offline'}
          </Badge>
          <Switch
            checked={config.is_published}
            onCheckedChange={(v) => setConfig({ ...config, is_published: v })}
          />
        </div>
      </div>

      {/* ── Section: Branding ── */}
      <div className={cn("overflow-hidden rounded-lg border bg-card transition-shadow", isExpanded('branding') ? "border-primary/20" : "border-border/60")}>
        <button type="button" onClick={() => toggleSection('branding')} className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <PaintBoardIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Branding</p>
            <p className="text-xs text-muted-foreground">Logo, colors, and theme for your help center</p>
          </div>
          <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('branding') && 'rotate-180')} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('branding')}>
          <div>
          <div className="border-t border-border px-6 py-6 space-y-6">
          {/* Upload zones */}
          <div className="grid gap-6 sm:grid-cols-3">
            {/* Logo (light) upload zone */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Logo (Light)</Label>
              {config.brand_logo_url ? (
                <div className="group relative flex h-28 items-center justify-center rounded-lg border-2 border-dashed bg-muted/30 transition-colors hover:bg-muted/50">
                  <img src={config.brand_logo_url} alt="Logo" className="max-h-16 max-w-[160px] object-contain" />
                  <div className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-background/80 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button type="button" variant="outline" size="sm" disabled={uploadingLogo} onClick={() => logoInputRef.current?.click()}>
                      {uploadingLogo ? 'Uploading...' : 'Replace'}
                    </Button>
                    <Button type="button" variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setConfig({ ...config, brand_logo_url: '' })}>
                      Remove
                    </Button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  disabled={uploadingLogo}
                  onClick={() => logoInputRef.current?.click()}
                  className="flex h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed bg-muted/20 text-muted-foreground transition-colors hover:border-primary/30 hover:bg-muted/40 hover:text-foreground"
                >
                  <Image01Icon className="h-6 w-6" />
                  <span className="text-xs">{uploadingLogo ? 'Uploading...' : '200 × 50 px · SVG or PNG'}</span>
                </button>
              )}
              <input ref={logoInputRef} type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" className="hidden" onChange={(e) => handleAssetUpload(e, 'logo', setUploadingLogo, 'brand_logo_url')} />
            </div>
            {/* Logo (dark) upload zone */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Logo (Dark)</Label>
              {config.brand_logo_dark_url ? (
                <div className="group relative flex h-28 items-center justify-center rounded-lg border-2 border-dashed bg-secondary transition-colors hover:bg-secondary/80">
                  <img src={config.brand_logo_dark_url} alt="Logo (dark)" className="max-h-16 max-w-[160px] object-contain" />
                  <div className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-secondary/80 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button type="button" variant="outline" size="sm" disabled={uploadingLogoDark} onClick={() => logoDarkInputRef.current?.click()}>
                      {uploadingLogoDark ? 'Uploading...' : 'Replace'}
                    </Button>
                    <Button type="button" variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setConfig({ ...config, brand_logo_dark_url: '' })}>
                      Remove
                    </Button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  disabled={uploadingLogoDark}
                  onClick={() => logoDarkInputRef.current?.click()}
                  className="flex h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed bg-secondary text-muted-foreground transition-colors hover:border-primary/30 hover:bg-secondary/80 hover:text-foreground"
                >
                  <Image01Icon className="h-6 w-6" />
                  <span className="text-xs">{uploadingLogoDark ? 'Uploading...' : '200 × 50 px · SVG or PNG'}</span>
                </button>
              )}
              <input ref={logoDarkInputRef} type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" className="hidden" onChange={(e) => handleAssetUpload(e, 'logo_dark', setUploadingLogoDark, 'brand_logo_dark_url')} />
            </div>
            {/* Favicon upload zone */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Favicon</Label>
              {config.favicon_url ? (
                <div className="group relative flex h-28 items-center justify-center rounded-lg border-2 border-dashed bg-muted/30 transition-colors hover:bg-muted/50">
                  <img src={config.favicon_url} alt="Favicon" className="h-10 w-10 object-contain" />
                  <div className="absolute inset-0 flex items-center justify-center gap-2 rounded-lg bg-background/80 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button type="button" variant="outline" size="sm" disabled={uploadingFavicon} onClick={() => faviconInputRef.current?.click()}>
                      {uploadingFavicon ? 'Uploading...' : 'Replace'}
                    </Button>
                    <Button type="button" variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setConfig({ ...config, favicon_url: '' })}>
                      Remove
                    </Button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  disabled={uploadingFavicon}
                  onClick={() => faviconInputRef.current?.click()}
                  className="flex h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed bg-muted/20 text-muted-foreground transition-colors hover:border-primary/30 hover:bg-muted/40 hover:text-foreground"
                >
                  <Image01Icon className="h-5 w-5" />
                  <span className="text-xs">{uploadingFavicon ? 'Uploading...' : '32 × 32 px · ICO, PNG, or SVG'}</span>
                </button>
              )}
              <input ref={faviconInputRef} type="file" accept="image/png,image/jpeg,image/svg+xml,image/x-icon,image/vnd.microsoft.icon" className="hidden" onChange={(e) => handleAssetUpload(e, 'favicon', setUploadingFavicon, 'favicon_url')} />
            </div>
          </div>

          <Separator />

          {/* Brand Name + Color + Theme */}
          <div className="grid gap-5 sm:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="hc-brand-name">Brand Name</Label>
              <Input
                id="hc-brand-name"
                value={config.brand_name}
                onChange={(e) => setConfig({ ...config, brand_name: e.target.value })}
                placeholder="Your Company"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-brand-color">Brand Color</Label>
              <div className="flex items-center gap-2">
                <div className="relative shrink-0">
                  <div
                    className="h-9 w-9 rounded-md border shadow-sm cursor-pointer"
                    style={{ backgroundColor: config.brand_color }}
                  />
                  <input
                    type="color"
                    id="hc-brand-color"
                    value={config.brand_color}
                    onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                    className="absolute inset-0 cursor-pointer opacity-0"
                  />
                </div>
                <Input
                  value={config.brand_color}
                  onChange={(e) => setConfig({ ...config, brand_color: e.target.value })}
                  className="flex-1 font-mono text-sm"
                  placeholder="#3b82f6"
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="flex items-center gap-1.5">
                <Label htmlFor="hc-theme-mode">Theme Mode</Label>
                <TooltipProvider delayDuration={200}>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <InformationCircleIcon className="h-3.5 w-3.5 text-muted-foreground cursor-help" />
                    </TooltipTrigger>
                    <TooltipContent side="top" className="max-w-[260px] text-xs leading-relaxed">
                      <p><strong>Light</strong> — Forces light theme, hides toggle</p>
                      <p><strong>Dark</strong> — Forces dark theme, hides toggle</p>
                      <p><strong>System</strong> — Follows visitor&apos;s OS, shows toggle</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <Select
                value={config.theme_mode}
                onValueChange={(v) => setConfig({ ...config, theme_mode: v as HelpcenterThemeMode })}
              >
                <SelectTrigger id="hc-theme-mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="light">Light</SelectItem>
                  <SelectItem value="dark">Dark</SelectItem>
                  <SelectItem value="system">System (auto)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          </div>
          </div>
        </div>
      </div>

      {/* ── Section: Domain & SEO ── */}
      <div className={cn("overflow-hidden rounded-lg border bg-card transition-shadow", isExpanded('domain-seo') ? "border-primary/20" : "border-border/60")}>
        <button type="button" onClick={() => toggleSection('domain-seo')} className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <GlobeIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Domain & SEO</p>
            <p className="text-xs text-muted-foreground">URL configuration and search engine optimization</p>
          </div>
          <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('domain-seo') && 'rotate-180')} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('domain-seo')}>
          <div>
          <div className="border-t border-border px-6 py-6 space-y-6">
          <div className="grid gap-6 lg:grid-cols-2">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-subdomain">Help Center URL</Label>
              <div className="flex items-center">
                <Input
                  id="hc-subdomain"
                  value={config.subdomain}
                  onChange={(e) => setConfig({ ...config, subdomain: e.target.value })}
                  placeholder="yourcompany"
                  className="rounded-r-none"
                />
                <span className="flex h-9 shrink-0 items-center rounded-r-md border border-l-0 bg-muted/50 px-3 text-sm text-muted-foreground">.helpin.center</span>
              </div>
              <p className="text-[11px] text-muted-foreground">
                Used as your default URL when no custom domain is set.
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-custom-domain">Custom Domain</Label>
              <Input
                id="hc-custom-domain"
                value={config.custom_domain}
                onChange={(e) => setConfig({ ...config, custom_domain: e.target.value })}
                placeholder="help.yourcompany.com"
              />
              <div className="rounded-md border border-border/60 bg-muted/40 p-3 text-[11px] leading-relaxed text-muted-foreground">
                <p className="mb-1.5 font-medium text-foreground">DNS setup</p>
                <p>
                  Create a <span className="font-mono text-foreground">CNAME</span> record on your domain pointing to{' '}
                  <span className="font-mono text-foreground">helpcenter.helpin.ai</span>. SSL certificates are issued automatically once DNS resolves.
                </p>
                <p className="mt-1.5">
                  Example: <span className="font-mono text-foreground">help.yourcompany.com</span> →{' '}
                  <span className="font-mono text-foreground">helpcenter.helpin.ai</span>
                </p>
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-support-email">Support Email</Label>
              <Input
                id="hc-support-email"
                type="email"
                value={config.support_email}
                onChange={(e) => setConfig({ ...config, support_email: e.target.value })}
                placeholder="support@yourcompany.com"
              />
            </div>
          </div>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="hc-seo-title">Meta Title</Label>
              <Input
                id="hc-seo-title"
                value={config.seo_title}
                onChange={(e) => setConfig({ ...config, seo_title: e.target.value })}
                placeholder="Help Center - Your Company"
              />
              <p className="text-[11px] text-muted-foreground">{config.seo_title.length}/60 characters</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-seo-desc">Meta Description</Label>
              <Textarea
                id="hc-seo-desc"
                value={config.seo_description}
                onChange={(e) => setConfig({ ...config, seo_description: e.target.value })}
                placeholder="Find answers, guides, and documentation..."
                rows={3}
              />
              <p className="text-[11px] text-muted-foreground">{config.seo_description.length}/160 characters</p>
            </div>
          </div>
          </div>
          </div>
          </div>
        </div>
      </div>

      {/* ── Section: Homepage ── */}
      <div className={cn("overflow-hidden rounded-lg border bg-card transition-shadow", isExpanded('homepage') ? "border-primary/20" : "border-border/60")}>
        <button type="button" onClick={() => toggleSection('homepage')} className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <LayoutGridIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Homepage</p>
            <p className="text-xs text-muted-foreground">Hero section and featured content visitors see first</p>
          </div>
          <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('homepage') && 'rotate-180')} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('homepage')}>
          <div>
          <div className="border-t border-border px-6 py-6 space-y-5">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="hc-hero-title">Hero Title</Label>
              <Input
                id="hc-hero-title"
                value={config.homepage_hero_title}
                onChange={(e) => setConfig({ ...config, homepage_hero_title: e.target.value })}
                placeholder="How can we help?"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hc-search-placeholder">Search Placeholder</Label>
              <Input
                id="hc-search-placeholder"
                value={config.search_placeholder}
                onChange={(e) => setConfig({ ...config, search_placeholder: e.target.value })}
                placeholder="Search articles..."
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="hc-hero-subtitle">Hero Subtitle</Label>
            <Input
              id="hc-hero-subtitle"
              value={config.homepage_hero_subtitle}
              onChange={(e) => setConfig({ ...config, homepage_hero_subtitle: e.target.value })}
              placeholder="Search our knowledge base or browse topics below"
            />
          </div>

          <Separator />

          {/* Featured Cards */}
          <div className="space-y-3">
            <div>
              <Label className="text-sm">Featured Cards</Label>
              <p className="text-xs text-muted-foreground mt-0.5">
                Select a space to populate homepage cards from its collections.
              </p>
            </div>
            <Select value={homepageSpaceSlug} onValueChange={handleHomepageSpaceChange}>
              <SelectTrigger className="w-full sm:w-64">
                <SelectValue placeholder="Select a space..." />
              </SelectTrigger>
              <SelectContent>
                {spaces.map(s => (
                  <SelectItem key={s.id} value={s.slug}>
                    <span className="inline-flex items-center gap-1">
                      <StoredIcon name={s.icon} className="h-4 w-4 shrink-0" textClassName="" />
                      <span>{s.name}</span>
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {spaceCollections.length > 0 && (() => {
              // Build a (collection_id -> ancestor path) map for the
              // active space so we can show nested collections with
              // their full breadcrumb ("Parent / Middle"). Depth-0
              // collections get no path label.
              const homepageSpace = spaces?.find((s) => s.slug === homepageSpaceSlug);
              const pathOptions = homepageSpace
                ? buildCollectionTreeOptions(homepageSpace.id, spaceCollections)
                : [];
              const pathBySourceId = new Map<string, string>();
              for (const opt of pathOptions) {
                if (opt.depth === 0) continue;
                // The option path already ends with the collection's own
                // name; strip the trailing segment so the breadcrumb
                // label shows only ancestors.
                const segments = opt.path.split(' / ');
                if (segments.length > 1) {
                  pathBySourceId.set(opt.id, segments.slice(0, -1).join(' / '));
                }
              }
              return (
                <DndContext sensors={dndSensors} collisionDetection={closestCenter} onDragEnd={handleFeaturedCardsDragEnd}>
                  <SortableContext items={selectedFeaturedCollectionIds} strategy={verticalListSortingStrategy}>
                    <div className="space-y-1.5">
                      {orderedSpaceCollections.map(col => {
                        const card = findFeaturedCardForCollection(
                          config.homepage_featured_cards,
                          col,
                          homepageSpaceSlug,
                        );
                        const checked = !!card;
                        const pathLabel = pathBySourceId.get(col.id);
                        if (checked && card) {
                          return (
                            <SortableFeaturedCollectionRow
                              key={col.id}
                              id={col.id}
                              collection={col}
                              card={card}
                              pathLabel={pathLabel}
                              onToggle={() => toggleCollection(col.id)}
                              onDescriptionChange={(value) => updateCardByCollectionId(col.id, { description: value })}
                              onIconChange={(value) => updateCardByCollectionId(col.id, { icon: value })}
                            />
                          );
                        }
                        return (
                          <div
                            key={col.id}
                            className={`flex items-center gap-3 rounded-lg border p-3 transition-colors ${checked ? 'border-primary/20 bg-primary/[0.03]' : 'border-transparent bg-muted/30'}`}
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={() => toggleCollection(col.id)}
                            />
                            <div className="min-w-0">
                              {pathLabel && (
                                <div className="text-[10px] text-muted-foreground/70 truncate">{pathLabel}</div>
                              )}
                              <span className="text-sm text-muted-foreground block truncate">{col.name}</span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </SortableContext>
                </DndContext>
              );
            })()}
          </div>
          </div>
          </div>
        </div>
      </div>

      {/* ── Section: Navigation ── */}
      <div className={cn("overflow-hidden rounded-lg border bg-card transition-shadow", isExpanded('navigation') ? "border-primary/20" : "border-border/60")}>
        <button type="button" onClick={() => toggleSection('navigation')} className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <Link01Icon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Navigation</p>
            <p className="text-xs text-muted-foreground">Header links and footer configuration</p>
          </div>
          <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('navigation') && 'rotate-180')} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('navigation')}>
          <div>
          <div className="border-t border-border px-6 py-6">
          <div className="grid gap-6 lg:grid-cols-2">
            {/* Header Links */}
            <div className="space-y-3 rounded-lg border border-border/60 p-4">
              <div>
                <Label className="text-sm font-medium">Header Links</Label>
                <p className="text-xs text-muted-foreground mt-0.5">Navigation links in the top bar.</p>
              </div>
              {config.header_links.length === 0 && (
                <p className="text-xs text-muted-foreground py-3 text-center">No header links yet.</p>
              )}
              {config.header_links.length > 0 && (
                <DndContext sensors={dndSensors} collisionDetection={closestCenter} onDragEnd={handleHeaderDragEnd}>
                  <SortableContext items={headerLinkIds} strategy={verticalListSortingStrategy}>
                    {config.header_links.map((link, i) => (
                      <SortableHeaderLinkRow
                        key={headerLinkIds[i]}
                        id={headerLinkIds[i]}
                        link={link}
                        onUpdate={(patch) => updateHeaderLink(i, patch)}
                        onRemove={() => removeHeaderLink(i)}
                      />
                    ))}
                  </SortableContext>
                </DndContext>
              )}
              <div className="flex justify-center">
                <Button type="button" variant="outline" size="sm" className="h-8 text-xs" onClick={addHeaderLink}>
                  <PlusSignIcon className="mr-1 h-3.5 w-3.5" /> Add Link
                </Button>
              </div>
            </div>

            {/* Footer */}
            <div className="space-y-4 rounded-lg border border-border/60 p-4">
              <div>
                <Label className="text-sm font-medium">Footer</Label>
                <p className="text-xs text-muted-foreground mt-0.5">Copyright text and footer links.</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="hc-footer-copyright" className="text-sm">Copyright Text</Label>
                <Input
                  id="hc-footer-copyright"
                  value={config.footer_copyright_text}
                  onChange={(e) => setConfig({ ...config, footer_copyright_text: e.target.value })}
                  placeholder={`\u00A9 ${new Date().getFullYear()} Your Company. All rights reserved.`}
                />
              </div>
              <div className="space-y-3">
                <Label className="text-sm">Footer Links</Label>
                {config.footer_links.length === 0 && (
                  <p className="text-xs text-muted-foreground py-2 text-center">No footer links yet.</p>
                )}
                {config.footer_links.length > 0 && (
                  <DndContext sensors={dndSensors} collisionDetection={closestCenter} onDragEnd={handleFooterDragEnd}>
                    <SortableContext items={config.footer_links.map((_, i) => i)} strategy={verticalListSortingStrategy}>
                      {config.footer_links.map((link, i) => (
                        <SortableFooterLinkRow
                          key={i}
                          id={i}
                          link={link}
                          onUpdate={(patch) => updateFooterLink(i, patch)}
                          onRemove={() => removeFooterLink(i)}
                        />
                      ))}
                    </SortableContext>
                  </DndContext>
                )}
                <div className="flex justify-center">
                  <Button type="button" variant="outline" size="sm" className="h-8 text-xs" onClick={addFooterLink}>
                    <PlusSignIcon className="mr-1 h-3.5 w-3.5" /> Add Link
                  </Button>
                </div>
              </div>
            </div>
          </div>
          </div>
          </div>
        </div>
      </div>

      {/* ── Section: Locales ── */}
      <div className={cn("overflow-hidden rounded-lg border bg-card transition-shadow", isExpanded('locales') ? "border-primary/20" : "border-border/60")}>
        <button type="button" onClick={() => toggleSection('locales')} className="flex w-full items-center gap-4 px-4 py-4 text-left transition-colors hover:bg-muted/40">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <LanguageCircleIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">Languages & Translation</p>
            <p className="text-xs text-muted-foreground">Manage supported languages and translation settings</p>
          </div>
          <ArrowDown01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200', isExpanded('locales') && 'rotate-180')} />
        </button>
        <div className="accordion-animate" data-open={isExpanded('locales')}>
          <div>
          <div className="border-t border-border px-6 py-6">
            <div className="space-y-6">
              <HelpcenterLocalesCard
                key={`${normalizedLocalesConfig.default_locale}:${normalizedLocalesConfig.enabled_locales.join(',')}:${String(normalizedLocalesConfig.show_language_switcher)}:${String(normalizedLocalesConfig.fallback_to_default_locale)}`}
                config={normalizedLocalesConfig}
                isSaving={updateLocales.isPending}
                onSave={async (data) => {
                  try {
                    await updateLocales.mutateAsync(data)
                    toast.success('Locale settings saved')
                  } catch (err) {
                    toast.error(err instanceof Error ? err.message : 'Failed to save locale settings')
                  }
                }}
              />

              <Separator />

              <div>
                <h3 className="text-sm font-medium">Protected terms</h3>
                <p className="text-xs text-muted-foreground mt-1">
                  Terms that AI will not translate — product names, features, and technical language.
                </p>
                <div className="mt-3">
                  <div className="space-y-3">
                    <div className="flex flex-wrap gap-2">
                      {config.protected_terms.map((term, i) => (
                        <span key={i} className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2 py-1 text-sm">
                          {term}
                          <button
                            type="button"
                            onClick={() => setConfig({ ...config, protected_terms: config.protected_terms.filter((_, j) => j !== i) })}
                            className="ml-0.5 text-muted-foreground hover:text-foreground transition-colors"
                          >
                            <Cancel01Icon className="h-3 w-3" />
                          </button>
                        </span>
                      ))}
                    </div>
                    <Input
                      placeholder="Type a term and press Enter..."
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          const val = (e.target as HTMLInputElement).value.trim()
                          if (val && !config.protected_terms.includes(val)) {
                            setConfig({ ...config, protected_terms: [...config.protected_terms, val] })
                            ;(e.target as HTMLInputElement).value = ''
                          }
                        }
                      }}
                    />
                  </div>
                </div>
              </div>

              <Separator />

              <HelpcenterTranslationsTable
                workspaceId={workspaceId}
                defaultLocale={normalizedLocalesConfig.default_locale}
                enabledLocales={normalizedLocalesConfig.enabled_locales}
              />
            </div>
          </div>
          </div>
        </div>
      </div>

    </form>
  );
}
