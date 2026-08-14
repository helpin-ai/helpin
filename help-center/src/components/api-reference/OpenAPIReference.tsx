import { useCallback, useMemo, useState } from 'react'
import { Code2 } from 'lucide-react'
import { APIReferenceContent } from './APIReferenceContent'
import { APIReferenceSidebar } from './APIReferenceSidebar'
import { APIRequestPanel } from './APIRequestPanel'
import {
  asOpenAPISpec,
  collectOperations,
  type ParsedOperation,
} from './openapi'
import type { APIReference } from '@/lib/types'
import type { APIReferenceSummary } from '@/lib/types'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalAPIReferencePath } from '@/lib/locale'
import { prefixBasepath } from '@/lib/pathUtils'

interface OpenAPIReferenceProps {
  reference: APIReference
  references: APIReferenceSummary[]
  locale: string
  spaceSlug: string
  multilingualEnabled: boolean
}

function scrollToOperation(operation: ParsedOperation) {
  document
    .getElementById(operation.anchor)
    ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.replaceState(null, '', `#${operation.anchor}`)
}

export function OpenAPIReference({
  reference,
  references,
  locale,
  spaceSlug,
  multilingualEnabled,
}: OpenAPIReferenceProps) {
  const { basepath } = useDocsContext()
  const spec = useMemo(
    () => asOpenAPISpec(reference.specification),
    [reference.specification],
  )
  const operations = useMemo(() => collectOperations(spec), [spec])
  const [activeOperationId, setActiveOperationId] = useState(
    operations[0]?.id ?? '',
  )
  const activeOperation =
    operations.find((operation) => operation.id === activeOperationId) ??
    operations[0]

  const setActiveOperation = useCallback((operation: ParsedOperation) => {
    setActiveOperationId(operation.id)
  }, [])

  return (
    <main className="api-reference-page min-w-0 bg-background">
      {operations.length > 0 && (
        <div className="sticky top-[var(--hc-header-height)] z-20 space-y-2 border-b border-border/70 bg-background/95 px-4 py-2 backdrop-blur lg:hidden">
          {references.length > 1 && (
            <label className="flex items-center gap-2">
              <span className="w-[62px] shrink-0 text-[11px] font-semibold text-muted-foreground">
                Reference
              </span>
              <select
                value={reference.slug}
                onChange={(event) => {
                  const target = buildCanonicalAPIReferencePath(
                    multilingualEnabled,
                    locale,
                    spaceSlug,
                    event.target.value,
                  )
                  window.location.assign(prefixBasepath(basepath, target))
                }}
                className="h-8 min-w-0 flex-1 rounded-md border border-input bg-background px-2 text-[12px] outline-none focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
              >
                {references.map((item) => (
                  <option key={item.id} value={item.slug}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label className="flex items-center gap-2">
            <span className="w-[62px] shrink-0 text-[11px] font-semibold text-muted-foreground">
              Endpoint
            </span>
            <select
              value={activeOperation?.id}
              onChange={(event) => {
                const operation = operations.find(
                  (item) => item.id === event.target.value,
                )
                if (!operation) return
                setActiveOperation(operation)
                scrollToOperation(operation)
              }}
              className="h-8 min-w-0 flex-1 rounded-md border border-input bg-background px-2 text-[12px] outline-none focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
            >
              {operations.map((operation) => (
                <option key={operation.anchor} value={operation.id}>
                  {operation.method.toUpperCase()} · {operation.summary}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}

      <div className="flex min-w-0 items-start">
        <APIReferenceSidebar
          spec={spec}
          operations={operations}
          references={references}
          currentReferenceSlug={reference.slug}
          locale={locale}
          spaceSlug={spaceSlug}
          multilingualEnabled={multilingualEnabled}
          activeOperationId={activeOperation?.id ?? ''}
          onSelectOperation={setActiveOperation}
        />

        <div className="min-w-0 flex-1 xl:grid xl:grid-cols-[minmax(0,1fr)_390px]">
          <div className="min-w-0">
            {activeOperation && (
              <div className="px-5 pt-5 sm:px-8 xl:hidden">
                <details className="group rounded-xl border border-border/80 bg-card">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3">
                    <span className="inline-flex items-center gap-2 text-[12.5px] font-semibold">
                      <Code2 size={15} className="text-primary" />
                      Test {activeOperation.summary}
                    </span>
                    <span className="text-muted-foreground transition-transform group-open:rotate-90">
                      ›
                    </span>
                  </summary>
                  <div className="border-t border-border/70 p-3">
                    <APIRequestPanel
                      key={activeOperation.id}
                      spec={spec}
                      operation={activeOperation}
                      compact
                    />
                  </div>
                </details>
              </div>
            )}

            <APIReferenceContent
              name={reference.name}
              apiVersion={reference.api_version}
              openapiVersion={reference.openapi_version}
              spec={spec}
              operations={operations}
              onActiveOperation={setActiveOperation}
            />
          </div>

          {activeOperation && (
            <aside className="sticky top-[var(--hc-header-height)] hidden h-[calc(100vh-var(--hc-header-height))] overflow-y-auto xl:block">
              <APIRequestPanel
                key={activeOperation.id}
                spec={spec}
                operation={activeOperation}
              />
            </aside>
          )}
        </div>
      </div>
    </main>
  )
}
