import { renderMermaidSvg } from '@/lib/mermaidRenderer'

const MAX_CACHED_DIAGRAMS = 100
const renderedDiagramCache = new Map<string, string>()
const pendingDiagramRenders = new Map<string, Promise<string>>()

export function getCachedMermaidDiagram(source: string): string | undefined {
  const trimmed = source.trim()
  if (!trimmed) return undefined
  const svg = renderedDiagramCache.get(trimmed)
  if (!svg) return undefined

  // Refresh insertion order so frequently revisited diagrams stay cached.
  renderedDiagramCache.delete(trimmed)
  renderedDiagramCache.set(trimmed, svg)
  return svg
}

function cacheDiagram(source: string, svg: string) {
  renderedDiagramCache.set(source, svg)
  if (renderedDiagramCache.size <= MAX_CACHED_DIAGRAMS) return

  const oldestSource = renderedDiagramCache.keys().next().value
  if (oldestSource) renderedDiagramCache.delete(oldestSource)
}

export function renderMermaidDiagram(source: string): Promise<string> {
  const trimmed = source.trim()
  const cached = getCachedMermaidDiagram(trimmed)
  if (cached) return Promise.resolve(cached)

  const pending = pendingDiagramRenders.get(trimmed)
  if (pending) return pending

  const render = renderMermaidSvg(trimmed)
    .then((svg) => {
      cacheDiagram(trimmed, svg)
      return svg
    })
    .finally(() => pendingDiagramRenders.delete(trimmed))
  pendingDiagramRenders.set(trimmed, render)
  return render
}

export function areMermaidDiagramsReady(sources: string[]): boolean {
  return sources.every((source) => {
    const trimmed = source.trim()
    return !trimmed || renderedDiagramCache.has(trimmed)
  })
}

export async function preloadMermaidDiagrams(sources: string[]): Promise<void> {
  const uniqueSources = [...new Set(sources.map((source) => source.trim()).filter(Boolean))]
  await Promise.allSettled(uniqueSources.map(renderMermaidDiagram))
}
