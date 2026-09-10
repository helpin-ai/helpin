import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { useState, useRef, useEffect, useMemo, useCallback } from 'react'
import { Alert01Icon, ArrowDown01Icon, Copy01Icon, SourceCodeIcon, Tick01Icon, ViewIcon } from '@/lib/icons'
import { common } from 'lowlight'
import { MermaidBlock } from './MermaidBlock'
import { NwdiagBlock } from './NwdiagBlock'

const LANGUAGES = Array.from(new Set([...Object.keys(common), 'mermaid', 'nwdiag'])).sort()

const DISPLAY_NAMES: Record<string, string> = {
  bash: 'Bash',
  c: 'C',
  cpp: 'C++',
  csharp: 'C#',
  css: 'CSS',
  diff: 'Diff',
  go: 'Go',
  graphql: 'GraphQL',
  html: 'HTML',
  ini: 'INI',
  java: 'Java',
  javascript: 'JavaScript',
  json: 'JSON',
  kotlin: 'Kotlin',
  lua: 'Lua',
  makefile: 'Makefile',
  markdown: 'Markdown',
  mermaid: 'Mermaid',
  nwdiag: 'nwdiag',
  objectivec: 'Objective-C',
  php: 'PHP',
  plaintext: 'Plain text',
  python: 'Python',
  r: 'R',
  ruby: 'Ruby',
  rust: 'Rust',
  scss: 'SCSS',
  shell: 'Shell',
  sql: 'SQL',
  swift: 'Swift',
  typescript: 'TypeScript',
  xml: 'XML',
  yaml: 'YAML',
}

function displayName(lang: string): string {
  return DISPLAY_NAMES[lang] || lang
}

export function CodeBlockNodeView({ node, updateAttributes, extension }: NodeViewProps) {
  const language = (node.attrs.language as string) || extension.options.defaultLanguage || 'plaintext'
  const isNwdiag = language.toLowerCase() === 'nwdiag'
  const isDiagram = language.toLowerCase() === 'mermaid' || isNwdiag
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [copied, setCopied] = useState(false)
  const [diagramMode, setDiagramMode] = useState<'diagram' | 'source'>('diagram')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleCopy = useCallback(() => {
    const text = node.textContent
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }, [node])

  const filtered = useMemo(() => {
    if (!search) return LANGUAGES
    const q = search.toLowerCase()
    return LANGUAGES.filter(
      (l) => l.includes(q) || displayName(l).toLowerCase().includes(q),
    )
  }, [search])

  useEffect(() => {
    if (open) {
      setSearch('')
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  return (
    <NodeViewWrapper className="relative my-3">
      <div className="relative rounded-md border border-border bg-muted/50">
        <div className="flex items-center justify-end gap-1 px-3 py-1.5 border-b border-border/50" ref={dropdownRef}>
          {isDiagram && (
            <div className="mr-auto flex items-center gap-1 rounded border border-border/60 bg-background/70 p-0.5" contentEditable={false}>
              <button
                type="button"
                onClick={() => setDiagramMode('diagram')}
                className={`flex h-6 items-center gap-1 rounded px-2 text-xs transition-colors ${
                  diagramMode === 'diagram'
                    ? 'bg-accent text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                title="Show diagram"
              >
                <ViewIcon className="h-3 w-3" />
                Diagram
              </button>
              <button
                type="button"
                onClick={() => setDiagramMode('source')}
                className={`flex h-6 items-center gap-1 rounded px-2 text-xs transition-colors ${
                  diagramMode === 'source'
                    ? 'bg-accent text-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                title="Show source"
              >
                <SourceCodeIcon className="h-3 w-3" />
                Source
              </button>
            </div>
          )}
          <button
            type="button"
            onClick={handleCopy}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors px-1.5 py-0.5 rounded hover:bg-muted"
            contentEditable={false}
            title="Copy code"
          >
            {copied ? <Tick01Icon className="h-3 w-3" /> : <Copy01Icon className="h-3 w-3" />}
            {copied ? 'Copied' : 'Copy'}
          </button>
          <button
            type="button"
            onClick={() => setOpen(!open)}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors px-1.5 py-0.5 rounded hover:bg-muted"
            contentEditable={false}
          >
            {displayName(language)}
            <ArrowDown01Icon className="h-3 w-3" />
          </button>

          {open && (
            <div
              className="absolute top-full right-0 mt-1 z-50 w-48 rounded-md border border-border bg-popover shadow-md"
              contentEditable={false}
            >
              <div className="p-1.5">
                <input
                  ref={inputRef}
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') setOpen(false)
                    if (e.key === 'Enter' && filtered.length > 0) {
                      updateAttributes({ language: filtered[0] })
                      setOpen(false)
                    }
                  }}
                  placeholder="Search..."
                  className="w-full rounded border border-border bg-background px-2 py-1 text-xs outline-none focus:ring-1 focus:ring-ring"
                />
              </div>
              <div className="max-h-48 overflow-y-auto py-1">
                {filtered.map((lang) => (
                  <button
                    key={lang}
                    type="button"
                    onClick={() => {
                      updateAttributes({ language: lang })
                      setOpen(false)
                    }}
                    className={`w-full text-left px-3 py-1 text-xs hover:bg-accent transition-colors ${
                      lang === language ? 'text-foreground font-medium' : 'text-muted-foreground'
                    }`}
                  >
                    {displayName(lang)}
                  </button>
                ))}
                {filtered.length === 0 && (
                  <p className="px-3 py-1 text-xs text-muted-foreground">No languages found</p>
                )}
              </div>
            </div>
          )}
        </div>

        {isDiagram && diagramMode === 'diagram' && (
          <div contentEditable={false}>
            {node.textContent.trim() ? (
              isNwdiag ? <NwdiagBlock source={node.textContent} /> : <MermaidBlock source={node.textContent} />
            ) : (
              <div className="flex min-h-32 items-center gap-3 px-4 py-3 text-sm text-muted-foreground">
                <Alert01Icon className="h-4 w-4 shrink-0" />
                Add {isNwdiag ? 'nwdiag' : 'Mermaid'} source to render a diagram.
              </div>
            )}
          </div>
        )}

        <pre
          className="!m-0 !rounded-t-none !border-0"
          spellCheck={false}
          style={isDiagram && diagramMode === 'diagram' ? { display: 'none' } : undefined}
        >
          <NodeViewContent as={"code" as "div"} />
        </pre>
      </div>
    </NodeViewWrapper>
  )
}
