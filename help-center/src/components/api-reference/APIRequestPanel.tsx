import { useMemo, useState } from 'react'
import { Check, Copy, KeyRound, Play, RotateCcw } from 'lucide-react'
import { MethodBadge } from './MethodBadge'
import {
  buildCodeSample,
  buildRequestHeaders,
  buildRequestURL,
  getRequestBodySchema,
  getSecuritySchemes,
  initialRequestValues,
  resolveServerURL,
  type CodeLanguage,
  type OpenAPISpec,
  type ParsedOperation,
  type RequestValues,
} from './openapi'

interface APIRequestPanelProps {
  spec: OpenAPISpec
  operation: ParsedOperation
  compact?: boolean
}

interface APIResponse {
  status: number
  statusText: string
  duration: number
  body: string
}

const languageLabels: Record<CodeLanguage, string> = {
  curl: 'cURL',
  javascript: 'JavaScript',
  python: 'Python',
}

function authInputLabel(
  scheme: ReturnType<typeof getSecuritySchemes>[number] | undefined,
) {
  if (!scheme) return ''
  if (scheme.scheme.type === 'apiKey') return scheme.scheme.name || 'API key'
  if (scheme.scheme.type === 'http' && scheme.scheme.scheme === 'basic') {
    return 'Base64 credentials'
  }
  return 'Bearer token'
}

export function APIRequestPanel({
  spec,
  operation,
  compact = false,
}: APIRequestPanelProps) {
  const servers = spec.servers?.length ? spec.servers : [{ url: '' }]
  const [serverURL, setServerURL] = useState(() => resolveServerURL(servers[0]))
  const [values, setValues] = useState<RequestValues>(() =>
    initialRequestValues(spec, operation),
  )
  const [language, setLanguage] = useState<CodeLanguage>('curl')
  const [copied, setCopied] = useState(false)
  const [sending, setSending] = useState(false)
  const [response, setResponse] = useState<APIResponse | null>(null)
  const [requestError, setRequestError] = useState<string | null>(null)
  const requestBody = getRequestBodySchema(spec, operation.requestBody)
  const securitySchemes = getSecuritySchemes(spec, operation)
  const security =
    securitySchemes.find((item) => item.key === values.authSchemeKey) ??
    securitySchemes[0]
  const editableParameters = operation.parameters.filter((parameter) =>
    ['path', 'query', 'header'].includes(parameter.in),
  )

  const codeSample = useMemo(
    () => buildCodeSample(language, spec, operation, serverURL, values),
    [language, operation, serverURL, spec, values],
  )

  const updateParameter = (key: string, value: string) => {
    setValues((current) => ({
      ...current,
      parameters: { ...current.parameters, [key]: value },
    }))
  }

  const reset = () => {
    setServerURL(resolveServerURL(servers[0]))
    setValues(initialRequestValues(spec, operation))
    setResponse(null)
    setRequestError(null)
  }

  const copyCode = async () => {
    await navigator.clipboard.writeText(codeSample)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1500)
  }

  const sendRequest = async () => {
    setSending(true)
    setResponse(null)
    setRequestError(null)
    const startedAt = performance.now()
    try {
      const url = buildRequestURL(serverURL, operation, values)
      if (!/^https?:\/\//i.test(url)) {
        throw new Error('Choose a complete HTTP or HTTPS server URL.')
      }
      const headers = buildRequestHeaders(spec, operation, values)
      const includesBody =
        !!values.body.trim() && !['get', 'head'].includes(operation.method)
      const result = await fetch(url, {
        method: operation.method.toUpperCase(),
        headers,
        body: includesBody ? values.body : undefined,
      })
      const rawBody = await result.text()
      let formattedBody = rawBody
      try {
        formattedBody = JSON.stringify(JSON.parse(rawBody), null, 2)
      } catch {
        // Non-JSON responses are displayed verbatim.
      }
      setResponse({
        status: result.status,
        statusText: result.statusText,
        duration: Math.round(performance.now() - startedAt),
        body: formattedBody || '(empty response)',
      })
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'The request could not be sent.'
      setRequestError(
        message.includes('fetch')
          ? 'The browser blocked this request. Confirm the API allows CORS for this help-center domain.'
          : message,
      )
    } finally {
      setSending(false)
    }
  }

  return (
    <div
      className={
        compact
          ? 'rounded-xl border border-border/80 bg-card'
          : 'min-h-full border-l border-border/70 bg-card/60'
      }
    >
      <div className={compact ? 'p-4' : 'p-5'}>
        <div className="flex items-center gap-2">
          <MethodBadge method={operation.method} compact />
          <code className="min-w-0 truncate font-mono text-[11px] text-muted-foreground">
            {operation.path}
          </code>
        </div>

        <div className="mt-5">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-[12px] font-semibold">Build a request</h2>
            <button
              type="button"
              onClick={reset}
              className="inline-flex items-center gap-1 text-[10.5px] text-muted-foreground transition-colors hover:text-foreground"
            >
              <RotateCcw size={11} />
              Reset
            </button>
          </div>

          <label className="block">
            <span className="mb-1 block text-[10.5px] font-medium text-muted-foreground">
              Server
            </span>
            {servers.length > 1 ? (
              <select
                value={serverURL}
                onChange={(event) => setServerURL(event.target.value)}
                className="h-8 w-full rounded-md border border-input bg-background px-2 font-mono text-[11px] outline-none focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
              >
                {servers.map((server, index) => {
                  const url = resolveServerURL(server)
                  return (
                    <option key={`${url}-${index}`} value={url}>
                      {server.description || url || 'Custom server'}
                    </option>
                  )
                })}
              </select>
            ) : (
              <input
                value={serverURL}
                onChange={(event) => setServerURL(event.target.value)}
                placeholder="https://api.example.com"
                className="h-8 w-full rounded-md border border-input bg-background px-2 font-mono text-[11px] outline-none transition-shadow placeholder:text-muted-foreground/50 focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
              />
            )}
          </label>

          {security && (
            <label className="mt-3 block">
              <span className="mb-1 flex items-center gap-1 text-[10.5px] font-medium text-muted-foreground">
                <KeyRound size={10} />
                Authentication
              </span>
              {securitySchemes.length > 1 && (
                <select
                  value={security.key}
                  onChange={(event) =>
                    setValues((current) => ({
                      ...current,
                      authSchemeKey: event.target.value,
                      authValue: '',
                    }))
                  }
                  className="mb-2 h-8 w-full rounded-md border border-input bg-background px-2 text-[11px] outline-none focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
                  aria-label="Authentication method"
                >
                  {securitySchemes.map((item) => (
                    <option key={item.key} value={item.key}>
                      {item.key} — {authInputLabel(item)}
                    </option>
                  ))}
                </select>
              )}
              <input
                type="password"
                value={values.authValue}
                onChange={(event) =>
                  setValues((current) => ({
                    ...current,
                    authValue: event.target.value,
                  }))
                }
                autoComplete="off"
                placeholder={authInputLabel(security)}
                aria-label={authInputLabel(security)}
                className="h-8 w-full rounded-md border border-input bg-background px-2 font-mono text-[11px] outline-none transition-shadow placeholder:text-muted-foreground/50 focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
              />
              <p className="mt-1 text-[9.5px] leading-4 text-muted-foreground/70">
                Kept in this browser tab and never stored by Helpin.
              </p>
            </label>
          )}

          {editableParameters.length > 0 && (
            <div className="mt-4 space-y-3">
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.1em] text-muted-foreground/70">
                Parameters
              </p>
              {editableParameters.map((parameter) => {
                const key = `${parameter.in}:${parameter.name}`
                return (
                  <label key={key} className="block">
                    <span className="mb-1 flex items-center justify-between gap-2 text-[10.5px] font-medium">
                      <span className="min-w-0 truncate font-mono">
                        {parameter.name}
                        {parameter.required && (
                          <span className="ml-0.5 text-red-500">*</span>
                        )}
                      </span>
                      <span className="shrink-0 text-[9px] uppercase text-muted-foreground/60">
                        {parameter.in}
                      </span>
                    </span>
                    <input
                      value={values.parameters[key] ?? ''}
                      onChange={(event) =>
                        updateParameter(key, event.target.value)
                      }
                      className="h-8 w-full rounded-md border border-input bg-background px-2 font-mono text-[11px] outline-none transition-shadow focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
                    />
                  </label>
                )
              })}
            </div>
          )}

          {requestBody && (
            <label className="mt-4 block">
              <span className="mb-1 flex items-center justify-between text-[10.5px] font-medium">
                <span>Request body</span>
                <span className="font-mono text-[9px] text-muted-foreground/60">
                  {requestBody.mediaType}
                </span>
              </span>
              <textarea
                value={values.body}
                onChange={(event) =>
                  setValues((current) => ({
                    ...current,
                    body: event.target.value,
                  }))
                }
                spellCheck={false}
                rows={compact ? 7 : 9}
                className="w-full resize-y rounded-md border border-input bg-background p-2 font-mono text-[10.5px] leading-5 outline-none transition-shadow focus:border-primary/50 focus:ring-2 focus:ring-primary/10"
              />
            </label>
          )}

          <button
            type="button"
            disabled={sending}
            onClick={sendRequest}
            className="mt-4 inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-md bg-primary px-3 text-[11.5px] font-semibold text-primary-foreground transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Play size={12} fill="currentColor" />
            {sending ? 'Sending…' : 'Send request'}
          </button>

          {requestError && (
            <div className="mt-3 rounded-md border border-red-500/20 bg-red-500/5 px-3 py-2 text-[10.5px] leading-4 text-red-700 dark:text-red-400">
              {requestError}
            </div>
          )}

          {response && (
            <div className="mt-4 overflow-hidden rounded-lg border border-border/80 bg-background">
              <div className="flex items-center justify-between border-b border-border/70 px-3 py-2">
                <div className="flex items-center gap-2">
                  <span
                    className={`font-mono text-[11px] font-semibold ${
                      response.status >= 200 && response.status < 300
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-red-600 dark:text-red-400'
                    }`}
                  >
                    {response.status}
                  </span>
                  <span className="text-[10px] text-muted-foreground">
                    {response.statusText}
                  </span>
                </div>
                <span className="font-mono text-[9.5px] text-muted-foreground">
                  {response.duration} ms
                </span>
              </div>
              <pre className="max-h-64 overflow-auto p-3 font-mono text-[10px] leading-4 text-foreground/80">
                <code>{response.body}</code>
              </pre>
            </div>
          )}
        </div>

        <div className="mt-6 overflow-hidden rounded-lg border border-border/80 bg-foreground text-background">
          <div className="flex items-center justify-between border-b border-background/15 px-2 py-1.5">
            <div className="flex items-center gap-0.5">
              {(Object.keys(languageLabels) as CodeLanguage[]).map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setLanguage(item)}
                  className={`rounded px-2 py-1 text-[9.5px] font-medium transition-colors ${
                    language === item
                      ? 'bg-background/15 text-background'
                      : 'text-background/55 hover:text-background'
                  }`}
                >
                  {languageLabels[item]}
                </button>
              ))}
            </div>
            <button
              type="button"
              onClick={copyCode}
              aria-label="Copy code sample"
              className="rounded p-1 text-background/55 transition-colors hover:bg-background/10 hover:text-background"
            >
              {copied ? <Check size={12} /> : <Copy size={12} />}
            </button>
          </div>
          <pre className="max-h-[360px] overflow-auto p-3.5 font-mono text-[10px] leading-[1.65] text-background/85">
            <code>{codeSample}</code>
          </pre>
        </div>
      </div>
    </div>
  )
}
