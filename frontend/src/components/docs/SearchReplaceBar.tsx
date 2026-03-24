import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronRight, Replace, ReplaceAll, X } from 'lucide-react'
import { type Editor } from '@tiptap/react'
import { searchReplacePluginKey, type SearchReplaceState } from './SearchReplaceExtension'
import { QuickTooltip } from '@/components/ui/quick-tooltip'

interface SearchReplaceBarProps {
  editor: Editor
  showReplace: boolean
  onClose: () => void
}

export function SearchReplaceBar({ editor, showReplace: initialShowReplace, onClose }: SearchReplaceBarProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [replaceTerm, setReplaceTerm] = useState('')
  const [showReplace, setShowReplace] = useState(initialShowReplace)
  const [pluginState, setPluginState] = useState<SearchReplaceState>({
    searchTerm: '',
    replaceTerm: '',
    results: [],
    currentIndex: 0,
  })
  const searchInputRef = useRef<HTMLInputElement>(null)

  // Focus search input on mount and when toggled open.
  useEffect(() => {
    requestAnimationFrame(() => searchInputRef.current?.focus())
  }, [])

  // Sync with initial showReplace prop changes (Ctrl+F vs Ctrl+H).
  useEffect(() => {
    setShowReplace(initialShowReplace)
  }, [initialShowReplace])

  // Listen to editor state changes to get current plugin state.
  useEffect(() => {
    const handler = () => {
      const state = searchReplacePluginKey.getState(editor.state) as SearchReplaceState | undefined
      if (state) setPluginState(state)
    }
    editor.on('transaction', handler)
    return () => { editor.off('transaction', handler) }
  }, [editor])

  const handleSearchChange = useCallback(
    (value: string) => {
      setSearchTerm(value)
      editor.commands.setSearchTerm(value)
    },
    [editor],
  )

  const handleReplaceChange = useCallback(
    (value: string) => {
      setReplaceTerm(value)
      editor.commands.setReplaceTerm(value)
    },
    [editor],
  )

  const handleClose = useCallback(() => {
    editor.commands.clearSearch()
    onClose()
  }, [editor, onClose])

  const handleReplaceCurrent = useCallback(() => {
    editor.commands.replaceCurrent()
  }, [editor])

  const handleReplaceAll = useCallback(() => {
    editor.commands.replaceAll()
  }, [editor])

  const handleNext = useCallback(() => {
    editor.commands.nextSearchResult()
  }, [editor])

  const handlePrev = useCallback(() => {
    editor.commands.prevSearchResult()
  }, [editor])

  const { results, currentIndex } = pluginState
  const matchCount = results.length
  const matchLabel = matchCount > 0 ? `${currentIndex + 1} of ${matchCount}` : 'No results'

  return (
    <div className="sticky top-2 ml-auto z-50 flex items-start gap-1.5 rounded-lg border border-border bg-background px-3 py-2 shadow-md w-[360px]">
      {/* Expand/collapse replace toggle */}
      <button
        type="button"
        onClick={() => setShowReplace(!showReplace)}
        className="mt-1 text-muted-foreground hover:text-foreground transition-colors"
      >
        <ChevronRight className={`h-4 w-4 transition-transform ${showReplace ? 'rotate-90' : ''}`} />
      </button>

      <div className="flex flex-col gap-1.5 flex-1 min-w-0">
        {/* Search row */}
        <div className="flex items-center gap-1.5">
          <input
            ref={searchInputRef}
            type="text"
            value={searchTerm}
            onChange={(e) => handleSearchChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                if (e.shiftKey) handlePrev()
                else handleNext()
              }
              if (e.key === 'Escape') {
                e.preventDefault()
                handleClose()
              }
            }}
            placeholder="Search..."
            className="flex-1 min-w-0 rounded border border-border bg-background px-2 py-1 text-sm outline-none focus:ring-1 focus:ring-ring"
          />
          <span className="text-xs text-muted-foreground whitespace-nowrap min-w-[60px] text-center">
            {searchTerm ? matchLabel : ''}
          </span>
          <QuickTooltip label="Previous (Shift+Enter)">
            <button
              type="button"
              onClick={handlePrev}
              disabled={matchCount === 0}
              className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 transition-colors"
            >
              <ArrowUp className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <QuickTooltip label="Next (Enter)">
            <button
              type="button"
              onClick={handleNext}
              disabled={matchCount === 0}
              className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 transition-colors"
            >
              <ArrowDown className="h-4 w-4" />
            </button>
          </QuickTooltip>
          <button
            type="button"
            onClick={handleClose}
            className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Replace row */}
        {showReplace && (
          <div className="flex items-center gap-1.5">
            <input
              type="text"
              value={replaceTerm}
              onChange={(e) => handleReplaceChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  handleReplaceCurrent()
                }
                if (e.key === 'Escape') {
                  e.preventDefault()
                  handleClose()
                }
              }}
              placeholder="Replace..."
              className="flex-1 min-w-0 rounded border border-border bg-background px-2 py-1 text-sm outline-none focus:ring-1 focus:ring-ring"
            />
            <QuickTooltip label="Replace">
              <button
                type="button"
                onClick={handleReplaceCurrent}
                disabled={matchCount === 0}
                className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 transition-colors"
              >
                <Replace className="h-4 w-4" />
              </button>
            </QuickTooltip>
            <QuickTooltip label="Replace All">
              <button
                type="button"
                onClick={handleReplaceAll}
                disabled={matchCount === 0}
                className="p-1 rounded text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 transition-colors"
              >
                <ReplaceAll className="h-4 w-4" />
              </button>
            </QuickTooltip>
          </div>
        )}
      </div>
    </div>
  )
}
