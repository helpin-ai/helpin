import type { ComponentType, SVGProps } from 'react'
import {
  Airplay,
  Archive,
  Bookmark,
  Box,
  Calendar,
  ChartBar,
  CheckCircle,
  CircleDot,
  Cloud,
  Code,
  Cog,
  Compass,
  CreditCard,
  Database,
  Download,
  File,
  FileText,
  Filter,
  Flag,
  Folder,
  FolderOpen,
  Globe,
  HardDrive,
  Hash,
  Heart,
  Home,
  Image,
  Inbox,
  Info,
  Key,
  Layers,
  Layout,
  Link,
  List,
  Lock,
  Mail,
  Map,
  MessageCircle,
  Monitor,
  Package,
  Pen,
  Phone,
  Play,
  Plus,
  Puzzle,
  Rocket,
  Search,
  Send,
  Settings,
  Shield,
  ShoppingCart,
  Sparkles,
  Star,
  Tag,
  Target,
  Terminal,
  ThumbsUp,
  Trash,
  TrendingUp,
  Upload,
  User,
  Users,
  Video,
  Wand,
  Wrench,
  Zap,
} from 'lucide-react'

type IconComponent = ComponentType<SVGProps<SVGSVGElement> & { size?: number | string }>

/**
 * Static icon map — Phosphor kebab-case names mapped to Lucide components.
 * Each icon is tree-shaken individually (~500 bytes). No lazy loading,
 * no Suspense, renders synchronously during SSR.
 */
const ICON_MAP: Record<string, IconComponent> = {
  // Direct name matches
  airplay: Airplay,
  archive: Archive,
  bookmark: Bookmark,
  calendar: Calendar,
  cloud: Cloud,
  code: Code,
  compass: Compass,
  database: Database,
  download: Download,
  filter: Filter,
  flag: Flag,
  folder: Folder,
  'folder-open': FolderOpen,
  globe: Globe,
  hash: Hash,
  heart: Heart,
  home: Home,
  image: Image,
  inbox: Inbox,
  info: Info,
  key: Key,
  layers: Layers,
  layout: Layout,
  link: Link,
  list: List,
  lock: Lock,
  mail: Mail,
  map: Map,
  monitor: Monitor,
  package: Package,
  phone: Phone,
  play: Play,
  plus: Plus,
  rocket: Rocket,
  search: Search,
  send: Send,
  shield: Shield,
  star: Star,
  tag: Tag,
  target: Target,
  terminal: Terminal,
  trash: Trash,
  upload: Upload,
  user: User,
  users: Users,
  video: Video,
  zap: Zap,

  // Phosphor → Lucide name mappings
  'chart-bar': ChartBar,
  'chart-line': TrendingUp,
  'check-circle': CheckCircle,
  'credit-card': CreditCard,
  'circle-dashed': CircleDot,
  'file-text': FileText,
  file: File,
  'gear': Cog,
  'gear-six': Cog,
  'hard-drive': HardDrive,
  'hard-drives': HardDrive,
  'chat-circle': MessageCircle,
  'chat-text': MessageCircle,
  'chat-dots': MessageCircle,
  'chats': MessageCircle,
  'envelope': Mail,
  'envelope-simple': Mail,
  'magic-wand': Wand,
  'sparkle': Sparkles,
  'shopping-cart': ShoppingCart,
  'shopping-bag': ShoppingCart,
  'thumbs-up': ThumbsUp,
  'trending-up': TrendingUp,
  'pencil-simple': Pen,
  'pencil': Pen,
  'note-pencil': Pen,
  'wrench': Wrench,
  'puzzle-piece': Puzzle,
  'cube': Box,
  'box': Box,
  'settings': Settings,
  'sliders': Settings,
  'sliders-horizontal': Settings,
}

const TOKEN_ICON_MAP: Record<string, IconComponent> = {
  account: User,
  add: Plus,
  ai: Sparkles,
  analysis: ChartBar,
  api: Code,
  chart: ChartBar,
  code: Code,
  developer: Code,
  doc: FileText,
  docs: FileText,
  email: Mail,
  envelope: Mail,
  help: Info,
  inbox: Inbox,
  list: List,
  mail: Mail,
  optimize: Sparkles,
  pen: Pen,
  publish: Send,
  quill: Pen,
  rocket: Rocket,
  seo: TrendingUp,
  setup: Cog,
  shopify: ShoppingCart,
  start: Rocket,
  startup: Rocket,
  voice: MessageCircle,
  webhook: Link,
  webhooks: Link,
  wordpress: Globe,
  write: Pen,
}

const NORMALIZED_ALIAS_MAP: Record<string, IconComponent> = {
  'add-to-list': List,
  'ai-voice': MessageCircle,
  'chart-no-axes-column-increasing': ChartBar,
  'mail-account': Mail,
  'quill-write': Pen,
  'start-up': Rocket,
}

function normalizeIconName(name: string): string {
  return name
    .toLowerCase()
    .replace(/icon$/, '')
    .replace(/\d+/g, '')
    .replace(/[^a-z-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

export function getIconComponent(name: string): IconComponent | null {
  if (!name) return null
  const direct = ICON_MAP[name.toLowerCase()]
  if (direct) return direct

  const normalized = normalizeIconName(name)
  if (!normalized) return null

  const normalizedDirect = ICON_MAP[normalized]
  if (normalizedDirect) return normalizedDirect

  const aliased = NORMALIZED_ALIAS_MAP[normalized]
  if (aliased) return aliased

  const tokens = normalized.split('-')
  for (const token of tokens) {
    const tokenMatch = TOKEN_ICON_MAP[token]
    if (tokenMatch) return tokenMatch
  }

  return null
}
