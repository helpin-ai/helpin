import { useCallback, useEffect, useRef, useState } from 'react'
import { useEditor, EditorContent, type JSONContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { Markdown } from 'tiptap-markdown'
import {
  AlertCircleIcon,
  Tick01Icon,
  ArrowDown01Icon,
  SourceCodeIcon,
  Copy01Icon,
  LinkSquare01Icon,
  Link01Icon,
  Menu01Icon,
  Loading01Icon,
  Cancel01Icon,
} from '@/lib/icons'
import {
  CheckListIcon as ListOrderedIcon,
  FileDownIcon,
  FileUpIcon,
  Heading02Icon,
  Heading03Icon,
  Heading04Icon,
  QuoteDownIcon,
  TextBoldIcon,
  TextItalicIcon,
  TextUnderlineIcon,
} from '@/lib/icons'
import UnderlineExtension from '@tiptap/extension-underline'
import Subscript from '@tiptap/extension-subscript'
import Superscript from '@tiptap/extension-superscript'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableHeader } from '@tiptap/extension-table-header'
import { TableCell } from '@tiptap/extension-table-cell'
import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'
import { SlashMenuExtension, slashMenuPluginKey } from './SlashMenuExtension'
import { BlockIdExtension } from './BlockIdExtension'
import { SlashMenu } from './SlashMenu'
import { CalloutExtension } from './CalloutExtension'
import { VideoEmbedExtension } from './VideoEmbedExtension'
import { HtmlBlockExtension } from './HtmlBlockExtension'
import { CodeBlockExtension } from './CodeBlockExtension'
import { SearchReplaceExtension } from './SearchReplaceExtension'
import { SearchReplaceBar } from './SearchReplaceBar'
import { EmojiPickerPopover } from './EmojiPickerPopover'
import { InsertVideoDialog } from './InsertVideoDialog'
import { TableControls } from './TableControls'
import { BlockGapInserter } from './BlockGapInserter'
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload'
import { docsService } from '@/lib/services/docsService'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { SlugDisplay } from './SlugDisplay'
import { toast } from 'sonner'

// ── Toolbar button ──────────────────────────────────────────────────────────

function ToolbarButton({
  onClick,
  active,
  children,
  title,
}: {
  onClick: () => void
  active?: boolean
  children: React.ReactNode
  title: string
}) {
  return (
    <QuickTooltip label={title} side="bottom">
      <button
        type="button"
        onMouseDown={(e) => {
          e.preventDefault() // prevent losing selection
          onClick()
        }}
        className={`rounded p-2 transition-colors ${
          active
            ? 'bg-accent text-foreground'
            : 'text-muted-foreground hover:bg-accent hover:text-foreground'
        }`}
      >
        {children}
      </button>
    </QuickTooltip>
  )
}

// ── Save status indicator ───────────────────────────────────────────────────

type SaveStatus = 'idle' | 'saved' | 'saving' | 'unsaved'

export interface DocsEditingPresenceSignal {
  area: 'title' | 'body'
  section?: string
}

function formatLastSaved(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 60) return 'a few seconds ago'
  const minutes = Math.floor(seconds / 60)
  if (minutes === 1) return '1 minute ago'
  if (minutes < 60) return `${minutes} minutes ago`
  const hours = Math.floor(minutes / 60)
  if (hours === 1) return '1 hour ago'
  return `${hours} hours ago`
}

function SaveIndicator({ status, lastSavedAt }: { status: SaveStatus; lastSavedAt: Date | null }) {
  const [, setTick] = useState(0)

  // Re-render every 30s to update "last saved X ago"
  useEffect(() => {
    if (!lastSavedAt || status === 'saving') return
    const interval = setInterval(() => setTick((t) => t + 1), 30_000)
    return () => clearInterval(interval)
  }, [lastSavedAt, status])

  if (status === 'idle' && !lastSavedAt) return null

  switch (status) {
    case 'saving':
      return (
        <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
          <Loading01Icon className="h-3 w-3 animate-spin" />
          Saving...
        </span>
      )
    case 'saved':
      return (
        <span className="flex items-center gap-1 text-[11px] text-green-600">
          <Tick01Icon className="h-3 w-3" />
          Saved
        </span>
      )
    case 'unsaved':
      return (
        <span className="text-[11px] text-muted-foreground">Unsaved changes</span>
      )
    default:
      // idle but has lastSavedAt — show "Last saved X ago"
      if (lastSavedAt) {
        return (
          <span className="text-[11px] text-muted-foreground">
            Last saved {formatLastSaved(lastSavedAt)}
          </span>
        )
      }
      return null
  }
}

// ── Floating toolbar ────────────────────────────────────────────────────────

function FloatingToolbar({ editor }: {
  editor: ReturnType<typeof useEditor>
  uploadConfig?: EditorUploadConfig
  onInsertImage: () => void
}) {
  const toolbarRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)
  const [showLinkPopover, setShowLinkPopover] = useState(false)
  const [linkUrl, setLinkUrl] = useState('')
  const [linkNewTab, setLinkNewTab] = useState(true)
  const [showFormatMenu, setShowFormatMenu] = useState(false)
  const [linkOnlyMode, setLinkOnlyMode] = useState(false) // show just link popover, no toolbar
  const linkOnlyRef = useRef(false)
  const linkInputRef = useRef<HTMLInputElement>(null)
  const savedSelectionRef = useRef<{ from: number; to: number } | null>(null)

  useEffect(() => {
    if (!editor) return

    const updatePosition = () => {
      const { from, to, empty } = editor.state.selection

      // Don't show toolbar when an atom node is selected (htmlBlock, videoEmbed, image)
      // Also hide when cursor is inside a non-text block
      const nodeAtSel = editor.state.doc.nodeAt(from)
      if (nodeAtSel?.type.spec.atom || editor.isActive('htmlBlock') || editor.isActive('videoEmbed')) {
        setPos(null)
        return
      }

      // Cursor on a link (no selection) — show link-only popover
      // Only trigger when cursor is truly inside the link, not at the boundary
      const $pos = editor.state.doc.resolve(from)
      const linkMarkAtCursor = $pos.marks().find(m => m.type.name === 'link')
      const charBeforeHasLink = from > 0 && editor.state.doc.resolve(from - 1).marks().some(m => m.type.name === 'link')
      const isInsideLink = !!(linkMarkAtCursor && charBeforeHasLink)

      if (empty && isInsideLink) {
        const coords = editor.view.coordsAtPos(from)
        const toolbar = toolbarRef.current
        const toolbarWidth = toolbar?.offsetWidth ?? 384
        setPos({
          top: coords.bottom + window.scrollY + 4,
          left: coords.left + window.scrollX - toolbarWidth / 2,
        })

        if (!linkOnlyRef.current) {
          const href = editor.getAttributes('link').href ?? ''
          setLinkUrl(href)
          setLinkNewTab(true)
          const $from = editor.state.doc.resolve(from)
          const linkMark = $from.marks().find(m => m.type.name === 'link')
          if (linkMark) {
            let linkFrom = from, linkTo = from
            editor.state.doc.nodesBetween(Math.max(0, from - 200), Math.min(editor.state.doc.content.size, from + 200), (node, pos) => {
              if (node.isText && node.marks.some(m => m.type.name === 'link' && m.attrs.href === linkMark.attrs.href)) {
                if (pos <= from && pos + node.nodeSize >= from) {
                  linkFrom = pos
                  linkTo = pos + node.nodeSize
                }
              }
            })
            savedSelectionRef.current = { from: linkFrom, to: linkTo }
          }
          setShowLinkPopover(true)
          setLinkOnlyMode(true)
          linkOnlyRef.current = true
        }
        return
      }

      // Cursor moved away from link — close link-only popover and clear position
      if (linkOnlyRef.current) {
        setShowLinkPopover(false)
        setLinkOnlyMode(false)
        linkOnlyRef.current = false
        savedSelectionRef.current = null
        lastPosRef.current = null
      }

      if (empty || from === to) {
        setPos(null)
        if (!linkOnlyRef.current) {
          lastPosRef.current = null
          setShowFormatMenu(false)
        }
        return
      }

      const domSelection = window.getSelection()
      if (!domSelection || domSelection.rangeCount === 0) {
        setPos(null)
        return
      }

      const range = domSelection.getRangeAt(0)
      const rect = range.getBoundingClientRect()
      if (rect.width === 0) {
        setPos(null)
        return
      }

      const toolbar = toolbarRef.current
      const toolbarWidth = toolbar?.offsetWidth ?? 300

      setPos({
        top: rect.top + window.scrollY - 45,
        left: rect.left + window.scrollX + rect.width / 2 - toolbarWidth / 2,
      })
    }

    editor.on('selectionUpdate', updatePosition)
    const handleBlur = () => {
      // Delay blur so clicks on toolbar/popover are processed first
      setTimeout(() => {
        // Don't hide if focus moved to our toolbar (e.g. link input)
        if (toolbarRef.current?.contains(document.activeElement)) return
        setPos(null)
        // Clear stale position only if no popover is open
        if (!linkOnlyRef.current) {
          lastPosRef.current = null
        }
      }, 150)
    }
    editor.on('blur', handleBlur)

    return () => {
      editor.off('selectionUpdate', updatePosition)
      editor.off('blur', handleBlur)
    }
  }, [editor])

  // Close popovers on click outside toolbar
  useEffect(() => {
    if (!showLinkPopover && !showFormatMenu) return
    const handleClick = (e: MouseEvent) => {
      if (toolbarRef.current?.contains(e.target as Node)) return
      setShowLinkPopover(false)
      setShowFormatMenu(false)
      savedSelectionRef.current = null
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [showLinkPopover, showFormatMenu])

  // Keep toolbar visible while link popover or format menu is open, even if selection is lost
  const lastPosRef = useRef(pos)
  if (pos) lastPosRef.current = pos
  const activePos = pos ?? lastPosRef.current

  if (!editor) return null
  if (!activePos && !showLinkPopover && !showFormatMenu) return null

  const openLinkPopover = () => {
    // Save selection before input steals focus
    const { from, to } = editor.state.selection
    savedSelectionRef.current = { from, to }
    const existing = editor.getAttributes('link').href ?? ''
    setLinkUrl(existing)
    setLinkNewTab(true)
    setShowLinkPopover(true)
    setShowFormatMenu(false)
    setTimeout(() => linkInputRef.current?.focus(), 50)
  }

  const normalizeUrl = (raw: string): string => {
    const s = raw.trim()
    if (!s) return ''
    // Already has protocol
    if (/^https?:\/\//i.test(s)) return s
    // Mailto
    if (/^mailto:/i.test(s)) return s
    // Tel
    if (/^tel:/i.test(s)) return s
    // Anchor link
    if (s.startsWith('#') || s.startsWith('/')) return s
    // Looks like a domain — add https
    return `https://${s}`
  }

  const isValidUrl = (raw: string): boolean => {
    const s = raw.trim()
    if (!s) return false
    // Anchor links and relative paths
    if (s.startsWith('#') || s.startsWith('/')) return true
    // Mailto and tel
    if (/^mailto:.+/i.test(s) || /^tel:.+/i.test(s)) return true
    // Must contain a dot for domain (e.g. google.com, docs.example.co.uk)
    const normalized = /^https?:\/\//i.test(s) ? s : `https://${s}`
    try {
      const url = new URL(normalized)
      return url.hostname.includes('.')
    } catch {
      return false
    }
  }

  const urlValid = isValidUrl(linkUrl)

  const applyLink = () => {
    const url = normalizeUrl(linkUrl)
    if (url && savedSelectionRef.current) {
      const { from, to } = savedSelectionRef.current
      editor.chain()
        .focus()
        .setTextSelection({ from, to })
        .setLink({
          href: url,
          target: linkNewTab ? '_blank' : null,
          rel: linkNewTab ? 'noopener noreferrer' : null,
        })
        .run()
    }
    setShowLinkPopover(false)
    setLinkOnlyMode(false)
    linkOnlyRef.current = false
    savedSelectionRef.current = null
  }

  const removeLink = () => {
    if (savedSelectionRef.current) {
      const { from, to } = savedSelectionRef.current
      editor.chain().focus().setTextSelection({ from, to }).unsetLink().run()
    } else {
      editor.chain().focus().unsetLink().run()
    }
    savedSelectionRef.current = null
    setShowLinkPopover(false)
    setLinkOnlyMode(false)
    linkOnlyRef.current = false
  }

  return (
    <div
      ref={toolbarRef}
      className="fixed z-50 flex flex-col items-center gap-0 animate-in fade-in zoom-in-95 duration-150"
      style={{ top: activePos?.top ?? 0, left: activePos?.left ?? 0 }}
    >
      {!linkOnlyMode && <div className="flex items-center gap-0.5 rounded-xl border border-border/70 bg-background/95 px-1.5 py-1 text-foreground shadow-xl backdrop-blur-md">
        {/* Bold, Italic, Underline */}
        <ToolbarButton title="Bold" onClick={() => editor.chain().focus().toggleBold().run()} active={editor.isActive('bold')}>
          <TextBoldIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton title="Italic" onClick={() => editor.chain().focus().toggleItalic().run()} active={editor.isActive('italic')}>
          <TextItalicIcon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton title="Underline" onClick={() => editor.chain().focus().toggleUnderline().run()} active={editor.isActive('underline')}>
          <TextUnderlineIcon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-0.5 h-4 w-px bg-border" />

        {/* H2, H3, H4 */}
        <ToolbarButton title="Heading 2" onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()} active={editor.isActive('heading', { level: 2 })}>
          <Heading02Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton title="Heading 3" onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()} active={editor.isActive('heading', { level: 3 })}>
          <Heading03Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton title="Heading 4" onClick={() => editor.chain().focus().toggleHeading({ level: 4 }).run()} active={editor.isActive('heading', { level: 4 })}>
          <Heading04Icon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-0.5 h-4 w-px bg-border" />

        {/* Lists */}
        <ToolbarButton title="Bullet list" onClick={() => editor.chain().focus().toggleBulletList().run()} active={editor.isActive('bulletList')}>
          <Menu01Icon className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton title="Ordered list" onClick={() => editor.chain().focus().toggleOrderedList().run()} active={editor.isActive('orderedList')}>
          <ListOrderedIcon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-0.5 h-4 w-px bg-border" />

        {/* Link */}
        <ToolbarButton title="Link" onClick={openLinkPopover} active={editor.isActive('link') || showLinkPopover}>
          <Link01Icon className="h-4 w-4" />
        </ToolbarButton>

        <div className="mx-0.5 h-4 w-px bg-border" />

        {/* Format dropdown */}
        <ToolbarButton title="Format" onClick={() => { setShowFormatMenu(!showFormatMenu); setShowLinkPopover(false); }} active={showFormatMenu}>
          <ArrowDown01Icon className="h-4 w-4" />
        </ToolbarButton>
      </div>}

      {/* Link popover */}
      {showLinkPopover && (
        <div className="mt-1 w-96 rounded-lg border bg-popover p-3 shadow-lg space-y-2.5" onMouseDown={(e) => { if ((e.target as HTMLElement).tagName !== 'INPUT') e.preventDefault(); }}>
          <div className="flex items-center gap-1.5">
            <input
              ref={linkInputRef}
              type="text"
              value={linkUrl}
              onChange={(e) => setLinkUrl(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && urlValid) applyLink(); if (e.key === 'Escape') setShowLinkPopover(false); }}
              placeholder="Paste or type a URL (e.g. google.com)"
              className={`flex-1 rounded-md border bg-background px-2.5 py-1.5 text-sm outline-none focus:ring-2 ${linkUrl && !urlValid ? 'border-destructive focus:ring-destructive/30' : 'focus:ring-primary/30'}`}
            />
            {linkUrl && urlValid && (
              <QuickTooltip label="Preview link">
                <button
                  type="button"
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-accent cursor-pointer"
                  onClick={() => window.open(normalizeUrl(linkUrl), '_blank', 'noopener,noreferrer')}
                >
                  <LinkSquare01Icon className="h-4 w-4" />
                </button>
              </QuickTooltip>
            )}
          </div>
          {linkUrl && !urlValid && (
            <p className="flex items-center gap-1 text-xs text-destructive">
              <AlertCircleIcon className="h-3 w-3 shrink-0" />
              Please enter a valid URL (e.g. google.com, /page, #section)
            </p>
          )}
          <div className="flex items-center justify-between">
            <button
              type="button"
              className="flex items-center gap-1.5 text-sm cursor-pointer"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => setLinkNewTab(!linkNewTab)}
            >
              <span className={`flex h-4 w-4 items-center justify-center rounded-sm border transition-colors ${linkNewTab ? 'bg-primary border-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
                {linkNewTab && <Tick01Icon className="h-3 w-3" />}
              </span>
              <span>Open in new tab</span>
            </button>
            <div className="flex gap-1">
              {editor.isActive('link') && (
                <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={removeLink} className="rounded px-2 py-1 text-xs text-destructive hover:bg-destructive/10 cursor-pointer">Remove</button>
              )}
              <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={applyLink} disabled={!urlValid || !linkUrl} className="rounded bg-primary px-3 py-1 text-xs text-primary-foreground hover:bg-primary/90 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">Apply</button>
            </div>
          </div>
        </div>
      )}

      {/* Format dropdown menu */}
      {showFormatMenu && (
        <div className="mt-1 w-44 rounded-lg border bg-popover p-1 shadow-lg" onMouseDown={(e) => e.preventDefault()}>
          <FormatMenuItem label="Normal" active={!editor.isActive('blockquote') && !editor.isActive('codeBlock')} onClick={() => { editor.chain().focus().clearNodes().run(); setShowFormatMenu(false); }} />
          <FormatMenuItem label="Blockquote" icon={<QuoteDownIcon className="h-3.5 w-3.5" />} active={editor.isActive('blockquote')} onClick={() => { editor.chain().focus().toggleBlockquote().run(); setShowFormatMenu(false); }} />
          <FormatMenuItem label="Code Block" icon={<SourceCodeIcon className="h-3.5 w-3.5" />} active={editor.isActive('codeBlock')} onClick={() => { editor.chain().focus().toggleCodeBlock().run(); setShowFormatMenu(false); }} />
          <div className="my-1 h-px bg-border" />
          <FormatMenuItem label="Inline Code" active={editor.isActive('code')} onClick={() => { editor.chain().focus().toggleCode().run(); setShowFormatMenu(false); }} />
          <FormatMenuItem label="Strikethrough" active={editor.isActive('strike')} onClick={() => { editor.chain().focus().toggleStrike().run(); setShowFormatMenu(false); }} />
          <FormatMenuItem label="Subscript" active={editor.isActive('subscript')} onClick={() => { editor.chain().focus().toggleSubscript().run(); setShowFormatMenu(false); }} />
          <FormatMenuItem label="Superscript" active={editor.isActive('superscript')} onClick={() => { editor.chain().focus().toggleSuperscript().run(); setShowFormatMenu(false); }} />
        </div>
      )}
    </div>
  )
}

function FormatMenuItem({ label, icon, active, onClick }: { label: string; icon?: React.ReactNode; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors cursor-pointer ${active ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'}`}
      onClick={onClick}
    >
      {icon && <span className="w-4 shrink-0">{icon}</span>}
      {!icon && <span className="w-4 shrink-0" />}
      {label}
    </button>
  )
}

// ── Import / Export menu ───────────────────────────────────────────────────

function ImportExportMenu({
  getMarkdown,
  onDownloadMarkdown,
  onDownloadDocx,
  onImportMarkdown,
  onUploadMarkdownFile,
  onUploadDocxFile,
  onToggleSource,
  sourceView,
}: {
  getMarkdown: () => string
  onDownloadMarkdown: () => void
  onDownloadDocx: () => void
  onImportMarkdown: () => void
  onUploadMarkdownFile: () => void
  onUploadDocxFile: () => void
  onToggleSource: () => void
  sourceView: boolean
}) {
  const [open, setOpen] = useState(false)

  const handleCopyMarkdown = (e: Event) => {
    e.preventDefault()
    const md = getMarkdown()
    if (!md) {
      toast.error('No content to copy')
      setOpen(false)
      return
    }
    // Intercept the copy event to inject our text directly into clipboardData
    const handler = (evt: ClipboardEvent) => {
      evt.clipboardData?.setData('text/plain', md)
      evt.preventDefault()
    }
    document.addEventListener('copy', handler, true)
    document.execCommand('copy')
    document.removeEventListener('copy', handler, true)
    setOpen(false)
    toast.success('Copied as Markdown')
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md bg-background/80 backdrop-blur-sm px-2 py-1 text-[11px] text-muted-foreground shadow-sm border border-border/40 transition-colors hover:bg-muted hover:text-foreground"
        >
          Import / Export
          <ArrowDown01Icon className="h-3 w-3" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuItem onSelect={handleCopyMarkdown}>
          <Copy01Icon className="h-3.5 w-3.5 mr-2" />
          Copy as Markdown
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onDownloadMarkdown}>
          <FileDownIcon className="h-3.5 w-3.5 mr-2" />
          Download as .md
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onDownloadDocx}>
          <FileDownIcon className="h-3.5 w-3.5 mr-2" />
          Download as .doc
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onImportMarkdown}>
          <FileUpIcon className="h-3.5 w-3.5 mr-2" />
          Import Markdown
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onUploadMarkdownFile}>
          <FileUpIcon className="h-3.5 w-3.5 mr-2" />
          Upload .md file
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onUploadDocxFile}>
          <FileUpIcon className="h-3.5 w-3.5 mr-2" />
          Upload .docx file
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onToggleSource}>
          <SourceCodeIcon className="h-3.5 w-3.5 mr-2" />
          {sourceView ? 'Back to Rich Editor' : 'Markdown Source'}
          <span className="ml-auto text-[10px] text-muted-foreground">
            {navigator.platform.includes('Mac') ? '⌘⇧M' : 'Ctrl+⇧+M'}
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ── Main editor ─────────────────────────────────────────────────────────────

interface DocsEditorProps {
  title?: string
  onTitleChange?: (title: string) => void
  slug?: string
  onSlugChange?: (slug: string) => Promise<void>
  slugHelperText?: string
  initialContent?: JSONContent | null
  onSave: (content: JSONContent) => Promise<void>
  autoSaveMs?: number
  readOnly?: boolean
  uploadConfig?: EditorUploadConfig
  topBanner?: React.ReactNode
  generatingOverlay?: string | null
  onEditingPresenceChange?: (presence: DocsEditingPresenceSignal | null) => void
}

export function DocsEditor({
  title,
  onTitleChange,
  slug,
  initialContent,
  onSave,
  autoSaveMs = 2000,
  readOnly = false,
  uploadConfig,
  topBanner,
  generatingOverlay,
  onSlugChange,
  slugHelperText,
  onEditingPresenceChange,
}: DocsEditorProps) {
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle')
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const savedFadeTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const savingRef = useRef(false)
  const pendingContentRef = useRef<JSONContent | null>(null)
  const skipNextSaveRef = useRef(false)
  const pendingImportedImageUploadsRef = useRef(new Set<string>())
  const importedImagePersistTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const lastSavedSnapshotRef = useRef<string | null>(
    initialContent ? JSON.stringify(initialContent) : null,
  )
  const editorReadyRef = useRef(false)
  const pendingPresenceClearRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const lastEditingPresenceRef = useRef<string | null>(null)

  // Markdown feature state
  const [sourceView, setSourceView] = useState(false)
  const [sourceMarkdown, setSourceMarkdown] = useState('')
  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const [videoDialogOpen, setVideoDialogOpen] = useState(false)
  const videoInsertPosRef = useRef<number>(0)
  const [emojiPickerOpen, setEmojiPickerOpen] = useState(false)
  const emojiInsertPosRef = useRef<number>(0)
  const [showSearch, setShowSearch] = useState(false)
  const [showSearchReplace, setShowSearchReplace] = useState(false)

  const doSave = useCallback(
    async (json: JSONContent) => {
      const snapshot = JSON.stringify(json)
      if (lastSavedSnapshotRef.current === snapshot) {
        setSaveStatus('idle')
        return
      }

      if (savingRef.current) {
        pendingContentRef.current = json
        return
      }
      savingRef.current = true
      setSaveStatus('saving')
      try {
        await onSave(json)
        lastSavedSnapshotRef.current = snapshot
        setSaveStatus('saved')
        setLastSavedAt(new Date())
        if (savedFadeTimerRef.current) clearTimeout(savedFadeTimerRef.current)
        savedFadeTimerRef.current = setTimeout(() => setSaveStatus('idle'), 5000)
      } catch {
        setSaveStatus('unsaved')
      } finally {
        savingRef.current = false
        if (pendingContentRef.current) {
          const next = pendingContentRef.current
          pendingContentRef.current = null
          doSave(next)
        }
      }
    },
    [onSave],
  )

  const scheduleSave = useCallback(
    (json: JSONContent) => {
      setSaveStatus('unsaved')
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
      saveTimerRef.current = setTimeout(() => doSave(json), autoSaveMs)
    },
    [doSave, autoSaveMs],
  )

  // Cleanup timers on unmount
  useEffect(() => () => {
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    if (savedFadeTimerRef.current) clearTimeout(savedFadeTimerRef.current)
    if (importedImagePersistTimerRef.current) clearTimeout(importedImagePersistTimerRef.current)
    if (pendingPresenceClearRef.current) clearTimeout(pendingPresenceClearRef.current)
  }, [])

  const emitEditingPresence = useCallback((presence: DocsEditingPresenceSignal | null) => {
    if (!onEditingPresenceChange || readOnly) return
    if (pendingPresenceClearRef.current) {
      clearTimeout(pendingPresenceClearRef.current)
      pendingPresenceClearRef.current = undefined
    }
    const nextKey = presence ? JSON.stringify(presence) : 'null'
    if (lastEditingPresenceRef.current === nextKey) return
    lastEditingPresenceRef.current = nextKey
    onEditingPresenceChange(presence)
  }, [onEditingPresenceChange, readOnly])

  const scheduleClearEditingPresence = useCallback(() => {
    if (!onEditingPresenceChange) return
    if (pendingPresenceClearRef.current) clearTimeout(pendingPresenceClearRef.current)
    pendingPresenceClearRef.current = setTimeout(() => {
      emitEditingPresence(null)
    }, 120)
  }, [emitEditingPresence, onEditingPresenceChange])

  const getNearestHeadingLabel = useCallback((editorInstance: NonNullable<typeof editorRef.current>) => {
    const selectionFrom = editorInstance.state.selection.from
    let headingText: string | undefined
    editorInstance.state.doc.nodesBetween(0, selectionFrom, (node) => {
      if (node.type.name === 'heading') {
        const text = node.textContent.trim()
        if (text) {
          headingText = text.length > 64 ? `${text.slice(0, 61)}...` : text
        }
      }
    })
    return headingText
  }, [])

  const emitBodyEditingPresence = useCallback((editorInstance: NonNullable<typeof editorRef.current>) => {
    emitEditingPresence({
      area: 'body',
      section: getNearestHeadingLabel(editorInstance),
    })
  }, [emitEditingPresence, getNearestHeadingLabel])

  useEffect(() => {
    if (!readOnly) return
    emitEditingPresence(null)
  }, [emitEditingPresence, readOnly])

  useEffect(() => {
    return () => {
      if (!onEditingPresenceChange || readOnly) return
      onEditingPresenceChange(null)
    }
  }, [onEditingPresenceChange, readOnly])

  const uploadConfigRef = useRef(uploadConfig)
  uploadConfigRef.current = uploadConfig
  const editorRef = useRef<ReturnType<typeof useEditor>>(null)

  const handleImageUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>) => {
      if (!editorInstance || !uploadConfigRef.current) return
      if (!file.type.startsWith('image/')) return

      const uploadId = `img-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

      // Insert a placeholder with data-uri
      const reader = new FileReader()
      reader.onload = async () => {
        const dataUri = reader.result as string
        editorInstance
          .chain()
          .focus()
          .setResizableImage({ src: dataUri, alt: file.name, title: uploadId })
          .run()

        try {
          const upload = await uploadEditorImage(file, uploadConfigRef.current!)

          // Replace data-uri with permanent URL
          const { doc } = editorInstance.state
          let targetPos: number | null = null
          doc.descendants((node, pos) => {
            if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
              targetPos = pos
              return false
            }
          })

          if (targetPos !== null) {
            const tr = editorInstance.state.tr
            const node = doc.nodeAt(targetPos)
            if (node) {
              tr.setNodeMarkup(targetPos, undefined, {
                ...node.attrs,
                src: upload.publicUrl,
                title: null,
                attachmentId: upload.attachmentId,
              })
              editorInstance.view.dispatch(tr)
            }
          }
        } catch (err) {
          toast.error(err instanceof Error ? err.message : 'Failed to upload image')
        }
      }
      reader.readAsDataURL(file)
    },
    [],
  )

  const persistImportedImages = useCallback(
    async (editorInstance: NonNullable<typeof editorRef.current>) => {
      const currentUploadConfig = uploadConfigRef.current

      const candidates: Array<{ pos: number; src: string; pendingId: string }> = []

      editorInstance.state.doc.descendants((node, pos) => {
        if (node.type.name !== 'resizableImage') return
        const src = typeof node.attrs.src === 'string' ? node.attrs.src : ''
        const attachmentId = typeof node.attrs.attachmentId === 'string' ? node.attrs.attachmentId : ''
        const isEmbeddedImage = src.startsWith('data:image/')
        const isRemoteImage = /^https?:\/\//i.test(src)
        if ((!isEmbeddedImage && !isRemoteImage) || attachmentId) return
        if (pendingImportedImageUploadsRef.current.has(src)) return
        if (isEmbeddedImage && !currentUploadConfig) return

        const pendingId = `imported-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
        pendingImportedImageUploadsRef.current.add(src)
        candidates.push({ pos, src, pendingId })
      })

      if (candidates.length === 0) return

      candidates.forEach(({ pos, pendingId }) => {
        const node = editorInstance.state.doc.nodeAt(pos)
        if (!node) return
        editorInstance.view.dispatch(
          editorInstance.state.tr.setNodeMarkup(pos, undefined, {
            ...node.attrs,
            title: pendingId,
          }),
        )
      })

      await Promise.all(
        candidates.map(async ({ src, pendingId }) => {
          try {
            let uploadedSrc = ''
            let uploadedAttachmentId: string | null = null

            if (src.startsWith('data:image/')) {
              const response = await fetch(src)
              const blob = await response.blob()
              if (!blob.type.startsWith('image/')) {
                throw new Error('Only image files are supported')
              }

              const extension = blob.type.split('/')[1]?.split('+')[0] || 'png'
              const file = new File([blob], `imported-image.${extension}`, { type: blob.type })
              const upload = await uploadEditorImage(file, currentUploadConfig!)
              uploadedSrc = upload.publicUrl
              uploadedAttachmentId = upload.attachmentId
            } else {
              if (!currentUploadConfig) {
                throw new Error('Editor upload is not configured')
              }
              const imported = await docsService.importExternalImage(currentUploadConfig.workspaceId, src)
              if (imported.error || !imported.data?.url) {
                throw new Error(imported.error || 'Failed to import external image')
              }
              uploadedSrc = imported.data.url
            }

            let targetPos: number | null = null
            editorInstance.state.doc.descendants((node, pos) => {
              if (node.type.name === 'resizableImage' && node.attrs.title === pendingId) {
                targetPos = pos
                return false
              }
            })

            if (targetPos !== null) {
              const node = editorInstance.state.doc.nodeAt(targetPos)
              if (node) {
                editorInstance.view.dispatch(
                  editorInstance.state.tr.setNodeMarkup(targetPos, undefined, {
                    ...node.attrs,
                    src: uploadedSrc,
                    title: null,
                    attachmentId: uploadedAttachmentId,
                  }),
                )
              }
            }
          } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to upload imported image')
          } finally {
            pendingImportedImageUploadsRef.current.delete(src)
          }
        }),
      )
    },
    [],
  )

  const queuePersistImportedImages = useCallback(
    (editorInstance: NonNullable<typeof editorRef.current>) => {
      if (importedImagePersistTimerRef.current) {
        clearTimeout(importedImagePersistTimerRef.current)
      }
      importedImagePersistTimerRef.current = setTimeout(() => {
        void persistImportedImages(editorInstance)
      }, 0)
    },
    [persistImportedImages],
  )

  const flushSave = useCallback(async () => {
    if (!editorRef.current || readOnly) return

    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current)
      saveTimerRef.current = undefined
    }

    const editorInstance = editorRef.current
    if (sourceView) {
      skipNextSaveRef.current = true
      editorInstance.commands.setContent(sourceMarkdown)
      void persistImportedImages(editorInstance)
    }

    await doSave(editorInstance.getJSON())
  }, [doSave, persistImportedImages, readOnly, sourceMarkdown, sourceView])

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3, 4, 5, 6] },
        codeBlock: false,
        link: {
          openOnClick: false,
          HTMLAttributes: { class: 'text-blue-600 dark:text-blue-400 underline cursor-pointer' },
        },
      }),
      CodeBlockExtension,
      Placeholder.configure({
        placeholder: ({ node }) => {
          if (node.type.name === 'heading') return 'Heading';
          if (node.type.name === 'codeBlock') return '';
          return "Type '/' for commands, or start writing...";
        },
      }),
      Markdown.configure({
        html: true,
        tightLists: true,
        bulletListMarker: '-',
        transformPastedText: true,
        transformCopiedText: false, // Don't force clipboard to markdown — we have explicit "Copy as Markdown"
      }),
      ResizableImageExtension,
      Table.configure({ resizable: true }),
      TableRow,
      TableHeader,
      TableCell,
      BlockIdExtension,
      SlashMenuExtension,
      CalloutExtension,
      VideoEmbedExtension,
      HtmlBlockExtension,
      UnderlineExtension,
      Subscript,
      Superscript,
      SearchReplaceExtension,
    ],
    content: initialContent ?? { type: 'doc', content: [{ type: 'paragraph' }] },
    editable: !readOnly,
    editorProps: {
      attributes: {
        class: 'docs-editor-prose prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[400px] px-6 pt-3 pb-8',
      },
      handlePaste(_view, event) {
        const items = event.clipboardData?.items
        if (!items) return false
        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault()
            const file = item.getAsFile()
            if (file && editorRef.current) {
              handleImageUpload(file, editorRef.current)
            }
            return true
          }
        }
        const html = event.clipboardData?.getData('text/html') ?? ''
        if (html.includes('<img') && editorRef.current) {
          queuePersistImportedImages(editorRef.current)
        }
        return false
      },
      handleDrop(_view, event) {
        const files = event.dataTransfer?.files
        if (!files?.length) return false
        for (const file of files) {
          if (file.type.startsWith('image/')) {
            event.preventDefault()
            if (editorRef.current) handleImageUpload(file, editorRef.current)
            return true
          }
        }
        const html = event.dataTransfer?.getData('text/html') ?? ''
        if (html.includes('<img') && editorRef.current) {
          queuePersistImportedImages(editorRef.current)
        }
        return false
      },
    },
    onUpdate: ({ editor: e }) => {
      // Skip saves during initial mount — TipTap fires onUpdate when normalizing content
      if (!editorReadyRef.current) return
      if (skipNextSaveRef.current) {
        skipNextSaveRef.current = false
        return
      }
      // Don't auto-save while slash menu is open — the /command text is transient
      const slashState = slashMenuPluginKey.getState(e.state) as any
      if (slashState?.open) return
      if (!readOnly) {
        scheduleSave(e.getJSON())
        emitBodyEditingPresence(e)
      }
    },
    onBlur: () => {
      scheduleClearEditingPresence()
    },
    onCreate: () => {
      // Mark editor ready after initialization is complete
      // Use requestAnimationFrame to ensure all mount-time updates have settled
      requestAnimationFrame(() => { editorReadyRef.current = true })
    },
  })

  editorRef.current = editor

  // Sync editable state when readOnly prop changes (e.g. after unlock)
  useEffect(() => {
    if (editor) {
      editor.setEditable(!readOnly)
    }
  }, [editor, readOnly])

  // Update content if initial content changes AFTER mount (e.g. after revert).
  // Skip the first run — useEditor already sets initial content on mount.
  const initialContentMountedRef = useRef(false)
  useEffect(() => {
    if (!editor || !initialContent) return
    if (!initialContentMountedRef.current) {
      initialContentMountedRef.current = true
      return
    }
    const currentJson = JSON.stringify(editor.getJSON())
    const newJson = JSON.stringify(initialContent)
    if (currentJson !== newJson) {
      skipNextSaveRef.current = true
      const { from, to } = editor.state.selection
      const wasFocused = editor.isFocused
      editor.commands.setContent(initialContent)
      if (wasFocused) {
        const maxPos = editor.state.doc.content.size
        editor.chain().focus().setTextSelection({
          from: Math.min(from, maxPos),
          to: Math.min(to, maxPos),
        }).run()
      }
      lastSavedSnapshotRef.current = newJson
      setSaveStatus('idle')
    }
  }, [editor, initialContent])

  const insertImage = useCallback(() => {
    if (!editor) return
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.onchange = () => {
      const file = input.files?.[0]
      if (file) {
        handleImageUpload(file, editor)
      }
    }
    input.click()
  }, [editor, handleImageUpload])

  // ── Markdown actions ───────────────────────────────────────────────────────

  const getMarkdown = useCallback((): string => {
    if (!editor) return ''
    return (editor.storage as Record<string, any>).markdown.getMarkdown()
  }, [editor])

  const handleImportMarkdown = useCallback(() => {
    if (!editor || !importText.trim()) return
    editor.commands.setContent(importText.trim())
    void persistImportedImages(editor)
    scheduleSave(editor.getJSON())
    setImportDialogOpen(false)
    setImportText('')
    toast.success('Markdown imported')
  }, [editor, importText, persistImportedImages, scheduleSave])

  const toggleSourceView = useCallback(() => {
    if (!editor) return
    if (!sourceView) {
      setSourceMarkdown(getMarkdown())
      setSourceView(true)
    } else {
      editor.commands.setContent(sourceMarkdown)
      void persistImportedImages(editor)
      scheduleSave(editor.getJSON())
      setSourceView(false)
    }
  }, [editor, sourceView, sourceMarkdown, getMarkdown, persistImportedImages, scheduleSave])

  const discardSourceView = useCallback(() => {
    setSourceView(false)
  }, [])

  // ── .md file download ──────────────────────────────────────────────────────

  const downloadAsMarkdown = useCallback(() => {
    const md = getMarkdown()
    const filename = `${(title || 'document').replace(/[^a-z0-9_-]/gi, '_').toLowerCase()}.md`
    const blob = new Blob([md], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }, [getMarkdown, title])

  // ── .md file upload ────────────────────────────────────────────────────────

  const uploadMarkdownFile = useCallback(() => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.md,.markdown,text/markdown'
    input.onchange = () => {
      const file = input.files?.[0]
      if (!file || !editor) return
      const reader = new FileReader()
      reader.onload = () => {
        const md = reader.result as string
        editor.commands.setContent(md)
        void persistImportedImages(editor)
        scheduleSave(editor.getJSON())
        toast.success(`Imported "${file.name}"`)
      }
      reader.onerror = () => toast.error('Failed to read file')
      reader.readAsText(file)
    }
    input.click()
  }, [editor, persistImportedImages, scheduleSave])

  // ── .docx export ──────────────────────────────────────────────────────────

  const downloadAsDocx = useCallback(() => {
    if (!editor) return
    const html = editor.getHTML()
    const docHtml = `<html xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:w="urn:schemas-microsoft-com:office:word" xmlns="http://www.w3.org/TR/REC-html40">
<head><meta charset="utf-8"><style>
body { font-family: Arial, sans-serif; font-size: 11pt; line-height: 1.6; color: #1a1a1a; }
h1 { font-size: 20pt; font-weight: bold; margin: 16pt 0 8pt; }
h2 { font-size: 16pt; font-weight: bold; margin: 14pt 0 6pt; }
h3 { font-size: 13pt; font-weight: bold; margin: 12pt 0 4pt; }
p { margin: 0 0 8pt; }
ul, ol { margin: 4pt 0 8pt 20pt; }
li { margin: 2pt 0; }
blockquote { border-left: 3pt solid #ccc; padding-left: 10pt; margin: 8pt 0; color: #555; }
code { font-family: Consolas, monospace; font-size: 10pt; background: #f4f4f4; padding: 1pt 3pt; }
pre { font-family: Consolas, monospace; font-size: 10pt; background: #f4f4f4; padding: 8pt; margin: 8pt 0; }
a { color: #1a73e8; }
img { max-width: 100%; }
</style></head>
<body>${html}</body></html>`
    const blob = new Blob([docHtml], { type: 'application/msword' })
    const filename = `${(title || 'document').replace(/[^a-z0-9_-]/gi, '_').toLowerCase()}.doc`
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }, [editor, title])

  // ── .docx import ──────────────────────────────────────────────────────────

  const uploadDocxFile = useCallback(() => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document'
    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file || !editor) return
      try {
        const mammoth = await import('mammoth')
        const arrayBuffer = await file.arrayBuffer()
        const result = await mammoth.convertToHtml({ arrayBuffer })
        editor.commands.setContent(result.value)
        void persistImportedImages(editor)
        scheduleSave(editor.getJSON())
        toast.success(`Imported "${file.name}"`)
        if (result.messages.length > 0) {
          const warnings = result.messages.filter((m) => m.type === 'warning').length
          if (warnings > 0) {
            toast.info(`${warnings} formatting warning${warnings > 1 ? 's' : ''} — some styles may have been simplified`)
          }
        }
      } catch {
        toast.error('Failed to import .docx file')
      }
    }
    input.click()
  }, [editor, persistImportedImages, scheduleSave])

  // ── Keyboard shortcuts ──────────────────────────────────────────────────────

  useEffect(() => {
    if (readOnly) return
    const handler = (e: KeyboardEvent) => {
      const key = e.key.toLowerCase()

      if ((e.ctrlKey || e.metaKey) && key === 's') {
        e.preventDefault()
        void flushSave()
        return
      }

      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'M') {
        e.preventDefault()
        toggleSourceView()
      }
      // Ctrl+F → search, Ctrl+H → search & replace
      if ((e.ctrlKey || e.metaKey) && key === 'f') {
        e.preventDefault()
        setShowSearch(true)
        setShowSearchReplace(false)
      }
      if ((e.ctrlKey || e.metaKey) && key === 'h') {
        e.preventDefault()
        setShowSearch(true)
        setShowSearchReplace(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [flushSave, readOnly, toggleSourceView])

  // ── Detect _markdown_source from backend AI import ─────────────────────────

  useEffect(() => {
    if (!editor || !initialContent) return
    const raw = initialContent as Record<string, unknown>
    if (typeof raw._markdown_source === 'string') {
      // Backend stored raw markdown — parse it via tiptap-markdown and auto-save as JSON
      editor.commands.setContent(raw._markdown_source as string)
      void persistImportedImages(editor)
      scheduleSave(editor.getJSON())
    }
  }, [editor, initialContent, persistImportedImages, scheduleSave])

  if (!editor) return null

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {/* Floating toolbar — appears on text selection */}
      {!readOnly && !sourceView && (
        <FloatingToolbar
          editor={editor}
          uploadConfig={uploadConfig}
          onInsertImage={insertImage}
        />
      )}

      {/* Editor content with title */}
      <div className={`relative min-h-0 flex-1 docs-editor-wrapper ${sourceView ? 'flex flex-col min-h-0' : 'overflow-y-auto'}`}>
        {generatingOverlay && (
          <div className="absolute inset-0 z-30 flex flex-col items-center justify-center bg-background/80 backdrop-blur-[2px]">
            <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground mb-3" />
            <p className="text-sm font-medium text-foreground">{generatingOverlay}</p>
          </div>
        )}
        {/* Markdown menu (left) + Save indicator (right) — floating */}
        {!readOnly && (
          <div className="sticky top-2 z-10 flex items-center justify-between px-4 pointer-events-none">
            <div className="pointer-events-auto">
              <ImportExportMenu
                getMarkdown={getMarkdown}
                onDownloadMarkdown={downloadAsMarkdown}
                onDownloadDocx={downloadAsDocx}
                onImportMarkdown={() => setImportDialogOpen(true)}
                onUploadMarkdownFile={uploadMarkdownFile}
                onUploadDocxFile={uploadDocxFile}
                onToggleSource={toggleSourceView}
                sourceView={sourceView}
              />
            </div>
            <div className="pointer-events-auto rounded-md bg-background/80 backdrop-blur-sm px-2 py-0.5 shadow-sm border border-border/40">
              <SaveIndicator status={saveStatus} lastSavedAt={lastSavedAt} />
            </div>
          </div>
        )}

        {topBanner}

        {sourceView ? (
          /* Source view — full width, fills remaining height */
          <div className="flex flex-1 flex-col px-6 py-4 min-h-0">
            {/* Title (read-only in source view) */}
            {title !== undefined && (
              <div className="pb-3 shrink-0">
                <h1 className="text-3xl font-bold text-left break-words">{title || 'Untitled'}</h1>
              </div>
            )}
            <div className="flex items-center justify-between mb-3 shrink-0">
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                Markdown Source
              </span>
              <div className="flex items-center gap-1.5">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 gap-1.5 text-xs"
                  onClick={discardSourceView}
                >
                  <Cancel01Icon className="h-3 w-3" />
                  Discard
                </Button>
                <Button
                  size="sm"
                  className="h-7 gap-1.5 text-xs"
                  onClick={toggleSourceView}
                >
                  <Tick01Icon className="h-3 w-3" />
                  Apply
                </Button>
              </div>
            </div>
            <textarea
              value={sourceMarkdown}
              onChange={(e) => setSourceMarkdown(e.target.value)}
              className="flex-1 w-full min-h-0 rounded-lg border border-border/60 bg-muted/30 p-4 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-ring resize-none"
              placeholder="Markdown content..."
              spellCheck={false}
            />
          </div>
        ) : (
          <div className="mx-auto max-w-3xl">
            {showSearch && editor && (
              <SearchReplaceBar
                editor={editor}
                showReplace={showSearchReplace}
                onClose={() => {
                  setShowSearch(false)
                  setShowSearchReplace(false)
                  editor.commands.focus()
                }}
              />
            )}
            {/* Title */}
            {title !== undefined && (
              <div className="group/title px-6 pt-10 pb-2">
                {slug && <SlugDisplay slug={slug} onSlugChange={onSlugChange} readOnly={readOnly} helperText={slugHelperText} />}
                {onTitleChange && !readOnly ? (
                  <textarea
                    value={title}
                    ref={(el) => {
                      if (el) {
                        el.style.height = 'auto'
                        el.style.height = `${el.scrollHeight}px`
                      }
                    }}
                    onChange={(e) => {
                      onTitleChange(e.target.value)
                      emitEditingPresence({ area: 'title', section: 'Title' })
                      e.target.style.height = 'auto'
                      e.target.style.height = `${e.target.scrollHeight}px`
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        editor?.commands.focus('start')
                      }
                    }}
                    onBlur={() => scheduleClearEditingPresence()}
                    placeholder="Untitled"
                    rows={1}
                    className="w-full bg-transparent text-3xl font-bold text-left outline-none placeholder:text-muted-foreground/40 resize-none overflow-hidden leading-tight break-words"
                  />
                ) : (
                  <h1 className="text-3xl font-bold text-left break-words">{title || 'Untitled'}</h1>
                )}
              </div>
            )}
            <EditorContent editor={editor} />
            {editor && (
              <>
                <SlashMenu
                  editor={editor}
                  onImageInsert={() => {
                    // Save cursor position before file picker steals focus
                    const savedPos = editor.state.selection.from;
                    const input = document.createElement('input');
                    input.type = 'file';
                    input.accept = 'image/*';
                    input.onchange = (e) => {
                      const file = (e.target as HTMLInputElement).files?.[0];
                      if (file && uploadConfig) {
                        // Restore cursor position before inserting
                        editor.chain().focus().setTextSelection(savedPos).run();
                        handleImageUpload(file, editor);
                      }
                    };
                    input.click();
                  }}
                  onVideoInsert={() => {
                    videoInsertPosRef.current = editor.state.selection.from;
                    setVideoDialogOpen(true);
                  }}
                  onEmojiInsert={() => {
                    emojiInsertPosRef.current = editor.state.selection.from;
                    setEmojiPickerOpen(true);
                  }}
                />
                <EmojiPickerPopover
                  editor={editor}
                  open={emojiPickerOpen}
                  onClose={() => setEmojiPickerOpen(false)}
                  insertPos={emojiInsertPosRef.current}
                />
                <TableControls editor={editor} />
                <BlockGapInserter editor={editor} />
              </>
            )}
            <div className="h-64" />
          </div>
        )}
      </div>

      {/* Insert Video dialog */}
      {editor && (
        <InsertVideoDialog
          open={videoDialogOpen}
          onOpenChange={setVideoDialogOpen}
          onInsert={(info) => {
            editor.chain()
              .focus()
              .setTextSelection(videoInsertPosRef.current)
              .setVideoEmbed({ provider: info.provider, sourceUrl: info.sourceUrl, embedUrl: info.embedUrl })
              .run();
            scheduleSave(editor.getJSON());
          }}
        />
      )}

      {/* Import Markdown dialog */}
      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Import Markdown</DialogTitle>
            <DialogDescription>
              Paste Markdown content below. It will replace the current document content.
            </DialogDescription>
          </DialogHeader>
          <textarea
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
            className="w-full min-h-[250px] rounded-lg border border-border/60 bg-muted/30 p-4 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-ring resize-y"
            placeholder="# Paste your Markdown here..."
            autoFocus
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => { setImportDialogOpen(false); setImportText('') }}>
              Cancel
            </Button>
            <Button onClick={handleImportMarkdown} disabled={!importText.trim()}>
              <FileDownIcon className="h-3.5 w-3.5 mr-1.5" />
              Import
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
