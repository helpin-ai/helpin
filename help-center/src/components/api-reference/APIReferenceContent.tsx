import { useEffect, useRef, useState } from 'react'
import { Bot, Check, ExternalLink, KeyRound, Server } from 'lucide-react'
import { MethodBadge } from './MethodBadge'
import {
  displaySchemaType,
  buildAIPrompt,
  formatExample,
  getRequestBodySchema,
  resolveReference,
  schemaExample,
  type OpenAPISpec,
  type ParsedOperation,
  type SchemaLike,
  type SchemaObject,
} from './openapi'

interface APIReferenceContentProps {
  name: string
  apiVersion: string
  openapiVersion: string
  spec: OpenAPISpec
  operations: ParsedOperation[]
  onActiveOperation: (operation: ParsedOperation) => void
}

function Description({ children }: { children?: string }) {
  if (!children) return null
  return (
    <div className="whitespace-pre-line text-[14px] leading-7 text-muted-foreground">
      {children}
    </div>
  )
}

function CopyForAIButton({
  spec,
  operations,
  apiVersion,
}: {
  spec: OpenAPISpec
  operations: ParsedOperation[]
  apiVersion: string
}) {
  const [copied, setCopied] = useState(false)

  const copyPrompt = async () => {
    const referenceURL = window.location.href.split('#')[0] ?? window.location.href
    const prompt = buildAIPrompt(spec, operations, referenceURL, apiVersion)
    await navigator.clipboard.writeText(prompt)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1800)
  }

  return (
    <button
      type="button"
      onClick={copyPrompt}
      className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-border bg-card px-2.5 text-[11.5px] font-semibold text-foreground shadow-sm transition-colors hover:border-primary/30 hover:bg-primary/5 hover:text-primary"
      title="Copy a ready-to-paste prompt with this API's reference and endpoint catalog"
    >
      {copied ? <Check size={13} /> : <Bot size={13} />}
      {copied ? 'Prompt copied' : 'Copy for AI'}
    </button>
  )
}

function SchemaProperties({
  spec,
  schemaLike,
}: {
  spec: OpenAPISpec
  schemaLike?: SchemaLike
}) {
  const schema = resolveReference<SchemaObject>(spec, schemaLike)
  const properties = Object.entries(schema?.properties ?? {})
  if (properties.length === 0) return null

  return (
    <div className="divide-y divide-border/70 rounded-lg border border-border/80">
      {properties.map(([name, property]) => {
        const resolved = resolveReference<SchemaObject>(spec, property)
        const required = schema?.required?.includes(name)
        return (
          <div key={name} className="grid gap-1 px-3.5 py-3 sm:grid-cols-[minmax(140px,0.42fr)_1fr] sm:gap-5">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-1.5">
                <code className="break-all font-mono text-[12.5px] font-semibold text-foreground">
                  {name}
                </code>
                {required && (
                  <span className="text-[10px] font-medium text-red-600 dark:text-red-400">
                    required
                  </span>
                )}
              </div>
              <p className="mt-0.5 break-all font-mono text-[11px] text-primary/80">
                {displaySchemaType(spec, property)}
              </p>
            </div>
            <div className="min-w-0 text-[12.5px] leading-5 text-muted-foreground">
              {resolved?.description || 'No description provided.'}
              {resolved?.default !== undefined && (
                <p className="mt-1">
                  Default:{' '}
                  <code className="rounded bg-muted px-1 py-0.5 font-mono text-[11px] text-foreground">
                    {String(resolved.default)}
                  </code>
                </p>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function APIEndpointSection({
  spec,
  operation,
  onActive,
}: {
  spec: OpenAPISpec
  operation: ParsedOperation
  onActive: (operation: ParsedOperation) => void
}) {
  const ref = useRef<HTMLElement>(null)
  const requestBody = getRequestBodySchema(spec, operation.requestBody)

  useEffect(() => {
    const node = ref.current
    if (!node || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) onActive(operation)
      },
      { rootMargin: '-20% 0px -65% 0px', threshold: 0 },
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [onActive, operation])

  const parameterGroups = ['path', 'query', 'header'].flatMap((location) => {
    const parameters = operation.parameters.filter(
      (parameter) => parameter.in === location,
    )
    return parameters.length > 0 ? [{ location, parameters }] : []
  })

  return (
    <section
      ref={ref}
      id={operation.anchor}
      className="scroll-mt-[calc(var(--hc-header-height)+28px)] border-t border-border/80 py-12 first:border-t-0"
      onFocusCapture={() => onActive(operation)}
      onMouseEnter={() => onActive(operation)}
    >
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <MethodBadge method={operation.method} />
        <code className="break-all font-mono text-[13px] font-medium text-foreground/80">
          {operation.path}
        </code>
        {operation.deprecated && (
          <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-400">
            Deprecated
          </span>
        )}
      </div>
      <h2 className="text-2xl font-semibold tracking-tight text-foreground">
        {operation.summary}
      </h2>
      {operation.description && (
        <div className="mt-3">
          <Description>{operation.description}</Description>
        </div>
      )}

      {parameterGroups.map(({ location, parameters }) => (
        <div key={location} className="mt-8">
          <h3 className="mb-3 text-[13px] font-semibold capitalize text-foreground">
            {location} parameters
          </h3>
          <div className="divide-y divide-border/70 rounded-lg border border-border/80">
            {parameters.map((parameter) => (
              <div key={`${location}:${parameter.name}`} className="grid gap-2 px-3.5 py-3.5 sm:grid-cols-[minmax(150px,0.42fr)_1fr] sm:gap-5">
                <div>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <code className="font-mono text-[12.5px] font-semibold text-foreground">
                      {parameter.name}
                    </code>
                    {parameter.required && (
                      <span className="text-[10px] font-medium text-red-600 dark:text-red-400">
                        required
                      </span>
                    )}
                  </div>
                  <p className="mt-0.5 break-all font-mono text-[11px] text-primary/80">
                    {displaySchemaType(spec, parameter.schema)}
                  </p>
                </div>
                <p className="text-[12.5px] leading-5 text-muted-foreground">
                  {parameter.description || 'No description provided.'}
                </p>
              </div>
            ))}
          </div>
        </div>
      ))}

      {requestBody && (
        <div className="mt-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-[13px] font-semibold text-foreground">
              Request body
            </h3>
            <span className="font-mono text-[10px] text-muted-foreground">
              {requestBody.mediaType}
            </span>
          </div>
          <SchemaProperties spec={spec} schemaLike={requestBody.schema} />
        </div>
      )}

      {operation.responses.length > 0 && (
        <div className="mt-8">
          <h3 className="mb-3 text-[13px] font-semibold text-foreground">
            Responses
          </h3>
          <div className="space-y-2">
            {operation.responses.map(({ status, response }) => {
              const content =
                response.content?.['application/json'] ??
                Object.values(response.content ?? {})[0]
              const responseSchema = content?.schema
              const example =
                content?.example ?? schemaExample(spec, responseSchema)
              return (
                <details
                  key={status}
                  className="group rounded-lg border border-border/80 bg-card"
                >
                  <summary className="flex cursor-pointer list-none items-center gap-3 px-3.5 py-3">
                    <span
                      className={`rounded px-1.5 py-0.5 font-mono text-[11px] font-semibold ${
                        status.startsWith('2')
                          ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
                          : status.startsWith('4') || status.startsWith('5')
                            ? 'bg-red-500/10 text-red-700 dark:text-red-400'
                            : 'bg-muted text-muted-foreground'
                      }`}
                    >
                      {status}
                    </span>
                    <span className="min-w-0 flex-1 text-[12.5px] text-muted-foreground">
                      {response.description || 'Response'}
                    </span>
                    <span className="text-xs text-muted-foreground transition-transform group-open:rotate-90">
                      ›
                    </span>
                  </summary>
                  {(responseSchema || example !== undefined) && (
                    <div className="border-t border-border/70 px-3.5 py-3.5">
                      <SchemaProperties spec={spec} schemaLike={responseSchema} />
                      {example !== undefined && (
                        <pre className="mt-3 max-w-full overflow-x-auto rounded-lg bg-foreground p-3.5 font-mono text-[11px] leading-5 text-background">
                          <code>{formatExample(example)}</code>
                        </pre>
                      )}
                    </div>
                  )}
                </details>
              )
            })}
          </div>
        </div>
      )}
    </section>
  )
}

function AuthenticationSection({ spec }: { spec: OpenAPISpec }) {
  const schemes = Object.entries(spec.components?.securitySchemes ?? {}).flatMap(
    ([name, value]) => {
      const scheme = resolveReference(spec, value)
      return scheme ? [{ name, scheme }] : []
    },
  )
  if (schemes.length === 0) return null

  return (
    <section
      id="api-authentication"
      className="scroll-mt-[calc(var(--hc-header-height)+28px)] border-t border-border/80 py-10"
    >
      <div className="mb-5 flex items-center gap-2">
        <KeyRound size={18} className="text-primary" />
        <h2 className="text-xl font-semibold tracking-tight">Authentication</h2>
      </div>
      <div className="grid gap-3">
        {schemes.map(({ name, scheme }) => (
          <div key={name} className="rounded-lg border border-border/80 bg-card p-4">
            <div className="flex flex-wrap items-center gap-2">
              <code className="font-mono text-[12.5px] font-semibold">{name}</code>
              <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                {scheme.type === 'http' ? scheme.scheme : scheme.type}
              </span>
            </div>
            {scheme.description && (
              <p className="mt-2 text-[12.5px] leading-5 text-muted-foreground">
                {scheme.description}
              </p>
            )}
            {scheme.type === 'apiKey' && (
              <p className="mt-2 font-mono text-[11px] text-muted-foreground">
                Send as {scheme.in}: {scheme.name}
              </p>
            )}
          </div>
        ))}
      </div>
    </section>
  )
}

function SchemasSection({ spec }: { spec: OpenAPISpec }) {
  const schemas = Object.entries(spec.components?.schemas ?? {})
  if (schemas.length === 0) return null

  return (
    <section className="border-t border-border/80 py-12">
      <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-primary">
        Models
      </p>
      <h2 className="mt-2 text-2xl font-semibold tracking-tight">Schemas</h2>
      <div className="mt-7 space-y-8">
        {schemas.map(([name, schema]) => {
          const resolved = resolveReference<SchemaObject>(spec, schema)
          return (
            <article
              key={name}
              id={`schema-${name}`}
              className="scroll-mt-[calc(var(--hc-header-height)+28px)]"
            >
              <div className="mb-3 flex min-w-0 flex-wrap items-center gap-2">
                <h3 className="min-w-0 break-all font-mono text-[15px] font-semibold">
                  {name}
                </h3>
                <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
                  {displaySchemaType(spec, schema)}
                </span>
              </div>
              {resolved?.description && (
                <p className="mb-3 text-[12.5px] leading-5 text-muted-foreground">
                  {resolved.description}
                </p>
              )}
              <SchemaProperties spec={spec} schemaLike={schema} />
            </article>
          )
        })}
      </div>
    </section>
  )
}

export function APIReferenceContent({
  name,
  apiVersion,
  openapiVersion,
  spec,
  operations,
  onActiveOperation,
}: APIReferenceContentProps) {
  const servers = spec.servers ?? []
  const title = spec.info?.title || name

  return (
    <article className="min-w-0 px-5 pb-20 pt-10 sm:px-8 xl:px-10">
      <div className="mx-auto w-full max-w-[760px]">
        <section
          id="api-overview"
          className="scroll-mt-[calc(var(--hc-header-height)+28px)] pb-10"
        >
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              {apiVersion && (
                <span className="rounded-full border border-border bg-card px-2.5 py-1 font-mono text-[10px] font-medium text-muted-foreground">
                  v{apiVersion.replace(/^v/i, '')}
                </span>
              )}
              <span className="rounded-full border border-border bg-card px-2.5 py-1 font-mono text-[10px] font-medium text-muted-foreground">
                OpenAPI {openapiVersion}
              </span>
            </div>
            <CopyForAIButton
              spec={spec}
              operations={operations}
              apiVersion={apiVersion}
            />
          </div>
          <h1 className="text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
            {title}
          </h1>
          {spec.info?.description && (
            <div className="mt-4 max-w-2xl">
              <Description>{spec.info.description}</Description>
            </div>
          )}

          {servers.length > 0 && (
            <div className="mt-8 rounded-lg border border-border/80 bg-card">
              <div className="flex items-center gap-2 border-b border-border/70 px-3.5 py-2.5">
                <Server size={14} className="text-primary" />
                <span className="text-[12px] font-semibold">Base URLs</span>
              </div>
              <div className="divide-y divide-border/70">
                {servers.map((server, index) => (
                  <div key={`${server.url}-${index}`} className="px-3.5 py-3">
                    <code className="break-all font-mono text-[12px] text-foreground">
                      {server.url}
                    </code>
                    {server.description && (
                      <p className="mt-1 text-[11.5px] text-muted-foreground">
                        {server.description}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {(spec.info?.termsOfService || spec.info?.license?.url) && (
            <div className="mt-5 flex flex-wrap gap-4 text-[12px]">
              {spec.info.termsOfService && (
                <a
                  href={spec.info.termsOfService}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-primary hover:underline"
                >
                  Terms of service <ExternalLink size={11} />
                </a>
              )}
              {spec.info.license?.url && (
                <a
                  href={spec.info.license.url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-primary hover:underline"
                >
                  {spec.info.license.name || 'License'} <ExternalLink size={11} />
                </a>
              )}
            </div>
          )}
        </section>

        <AuthenticationSection spec={spec} />

        <div>
          {operations.map((operation) => (
            <APIEndpointSection
              key={operation.anchor}
              spec={spec}
              operation={operation}
              onActive={onActiveOperation}
            />
          ))}
        </div>

        <SchemasSection spec={spec} />
      </div>
    </article>
  )
}
