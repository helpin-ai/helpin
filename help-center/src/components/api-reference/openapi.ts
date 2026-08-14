export const HTTP_METHODS = [
  'get',
  'post',
  'put',
  'patch',
  'delete',
  'options',
  'head',
  'trace',
] as const

export type HTTPMethod = (typeof HTTP_METHODS)[number]
export type CodeLanguage = 'curl' | 'javascript' | 'python'

export interface ReferenceObject {
  $ref: string
}

export interface SchemaObject {
  type?: string
  title?: string
  description?: string
  format?: string
  example?: unknown
  default?: unknown
  enum?: unknown[]
  nullable?: boolean
  required?: string[]
  properties?: Record<string, SchemaLike>
  items?: SchemaLike
  allOf?: SchemaLike[]
  oneOf?: SchemaLike[]
  anyOf?: SchemaLike[]
  additionalProperties?: boolean | SchemaLike
  [key: string]: unknown
}

export type SchemaLike = SchemaObject | ReferenceObject

export interface ParameterObject {
  name: string
  in: 'path' | 'query' | 'header' | 'cookie' | string
  description?: string
  required?: boolean
  example?: unknown
  schema?: SchemaLike
}

export interface RequestBodyObject {
  description?: string
  required?: boolean
  content?: Record<string, { schema?: SchemaLike; example?: unknown }>
}

export interface ResponseObject {
  description?: string
  content?: Record<string, { schema?: SchemaLike; example?: unknown }>
  headers?: Record<string, unknown>
}

export interface SecuritySchemeObject {
  type?: string
  scheme?: string
  bearerFormat?: string
  name?: string
  in?: string
  description?: string
  flows?: Record<string, unknown>
  openIdConnectUrl?: string
}

export interface OperationObject {
  operationId?: string
  summary?: string
  description?: string
  tags?: string[]
  deprecated?: boolean
  parameters?: Array<ParameterObject | ReferenceObject>
  requestBody?: RequestBodyObject | ReferenceObject
  responses?: Record<string, ResponseObject | ReferenceObject>
  security?: Array<Record<string, string[]>>
}

export interface PathItemObject {
  summary?: string
  description?: string
  parameters?: Array<ParameterObject | ReferenceObject>
  [method: string]: unknown
}

export interface OpenAPISpec {
  openapi?: string
  info?: {
    title?: string
    version?: string
    description?: string
    termsOfService?: string
    contact?: { name?: string; url?: string; email?: string }
    license?: { name?: string; url?: string }
  }
  servers?: Array<{
    url?: string
    description?: string
    variables?: Record<string, { default?: string }>
  }>
  tags?: Array<{ name?: string; description?: string }>
  paths?: Record<string, PathItemObject>
  components?: {
    schemas?: Record<string, SchemaLike>
    securitySchemes?: Record<string, SecuritySchemeObject | ReferenceObject>
  }
  security?: Array<Record<string, string[]>>
}

export interface ParsedOperation {
  id: string
  anchor: string
  path: string
  method: HTTPMethod
  summary: string
  description?: string
  tags: string[]
  deprecated: boolean
  parameters: ParameterObject[]
  requestBody?: RequestBodyObject
  responses: Array<{ status: string; response: ResponseObject }>
  security: Array<Record<string, string[]>>
}

export interface RequestValues {
  parameters: Record<string, string>
  body: string
  authSchemeKey: string
  authValue: string
}

export interface ResolvedSecurityScheme {
  key: string
  scheme: SecuritySchemeObject
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

export function isReference(value: unknown): value is ReferenceObject {
  return isRecord(value) && typeof value.$ref === 'string'
}

export function asOpenAPISpec(value: Record<string, unknown>): OpenAPISpec {
  return value as OpenAPISpec
}

export function resolveReference<T>(
  spec: OpenAPISpec,
  value: T | ReferenceObject | undefined,
): T | undefined {
  if (!value) return undefined
  if (!isReference(value)) return value as T
  if (!value.$ref.startsWith('#/')) return undefined

  const resolved = value.$ref
    .slice(2)
    .split('/')
    .map((part) => part.replace(/~1/g, '/').replace(/~0/g, '~'))
    .reduce<unknown>((current, key) => {
      if (!isRecord(current)) return undefined
      return current[key]
    }, spec)

  return resolved as T | undefined
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function resolveParameters(
  spec: OpenAPISpec,
  values: Array<ParameterObject | ReferenceObject> = [],
): ParameterObject[] {
  return values.flatMap((value) => {
    const resolved = resolveReference<ParameterObject>(spec, value)
    return resolved?.name && resolved.in ? [resolved] : []
  })
}

export function collectOperations(spec: OpenAPISpec): ParsedOperation[] {
  const operations: ParsedOperation[] = []

  Object.entries(spec.paths ?? {}).forEach(([path, pathItem]) => {
    const sharedParameters = resolveParameters(spec, pathItem.parameters)
    HTTP_METHODS.forEach((method) => {
      const rawOperation = pathItem[method]
      if (!isRecord(rawOperation)) return
      const operation = rawOperation as OperationObject
      const parameters = [
        ...sharedParameters,
        ...resolveParameters(spec, operation.parameters),
      ].filter(
        (parameter, index, all) =>
          all.findIndex(
            (candidate) =>
              candidate.name === parameter.name && candidate.in === parameter.in,
          ) === index,
      )
      const requestBody = resolveReference<RequestBodyObject>(
        spec,
        operation.requestBody,
      )
      const responses = Object.entries(operation.responses ?? {}).flatMap(
        ([status, value]) => {
          const response = resolveReference<ResponseObject>(spec, value)
          return response ? [{ status, response }] : []
        },
      )
      const tags = operation.tags?.filter(Boolean) ?? ['General']
      const anchor = `operation-${method}-${slugify(path) || 'root'}`

      operations.push({
        id: operation.operationId || `${method}-${path}`,
        anchor,
        path,
        method,
        summary:
          operation.summary ||
          operation.operationId ||
          `${method.toUpperCase()} ${path}`,
        description: operation.description,
        tags: tags.length > 0 ? tags : ['General'],
        deprecated: operation.deprecated === true,
        parameters,
        requestBody,
        responses,
        security: operation.security ?? spec.security ?? [],
      })
    })
  })

  return operations
}

export function groupOperations(
  spec: OpenAPISpec,
  operations: ParsedOperation[],
): Array<{ name: string; description?: string; operations: ParsedOperation[] }> {
  const tagDescriptions = new Map(
    (spec.tags ?? []).flatMap((tag) =>
      tag.name ? [[tag.name, tag.description] as const] : [],
    ),
  )
  const groups = new Map<string, ParsedOperation[]>()

  operations.forEach((operation) => {
    operation.tags.forEach((tag) => {
      const current = groups.get(tag) ?? []
      current.push(operation)
      groups.set(tag, current)
    })
  })

  return Array.from(groups, ([name, taggedOperations]) => ({
    name,
    description: tagDescriptions.get(name),
    operations: taggedOperations,
  }))
}

export function resolveServerURL(
  server: NonNullable<OpenAPISpec['servers']>[number] | undefined,
): string {
  let url = server?.url || ''
  Object.entries(server?.variables ?? {}).forEach(([name, variable]) => {
    url = url.replace(`{${name}}`, variable.default ?? '')
  })
  return url.replace(/\/$/, '')
}

export function getRequestBodySchema(
  spec: OpenAPISpec,
  requestBody: RequestBodyObject | undefined,
): { mediaType: string; schema?: SchemaObject; example?: unknown } | undefined {
  const entries = Object.entries(requestBody?.content ?? {})
  const entry =
    entries.find(([mediaType]) => mediaType === 'application/json') ?? entries[0]
  if (!entry) return undefined
  const [mediaType, content] = entry
  return {
    mediaType,
    schema: resolveReference<SchemaObject>(spec, content.schema),
    example: content.example,
  }
}

export function schemaExample(
  spec: OpenAPISpec,
  schemaLike: SchemaLike | undefined,
  depth = 0,
): unknown {
  if (!schemaLike || depth > 5) return undefined
  const schema = resolveReference<SchemaObject>(spec, schemaLike)
  if (!schema) return undefined
  if (schema.example !== undefined) return schema.example
  if (schema.default !== undefined) return schema.default
  if (schema.enum?.length) return schema.enum[0]

  const composite = schema.allOf ?? schema.oneOf ?? schema.anyOf
  if (composite?.length) {
    if (schema.allOf) {
      return Object.assign(
        {},
        ...composite.map((part) => schemaExample(spec, part, depth + 1)),
      )
    }
    return schemaExample(spec, composite[0], depth + 1)
  }

  if (schema.type === 'array' || schema.items) {
    const item = schemaExample(spec, schema.items, depth + 1)
    return item === undefined ? [] : [item]
  }
  if (schema.type === 'object' || schema.properties) {
    return Object.fromEntries(
      Object.entries(schema.properties ?? {}).map(([key, value]) => [
        key,
        schemaExample(spec, value, depth + 1),
      ]),
    )
  }
  if (schema.type === 'boolean') return true
  if (schema.type === 'integer' || schema.type === 'number') return 0
  if (schema.format === 'date-time') return '2026-01-01T00:00:00Z'
  if (schema.format === 'date') return '2026-01-01'
  if (schema.format === 'uuid') return '00000000-0000-4000-8000-000000000000'
  return 'string'
}

export function initialParameterValue(
  spec: OpenAPISpec,
  parameter: ParameterObject,
): string {
  const schema = resolveReference<SchemaObject>(spec, parameter.schema)
  const value =
    parameter.example ??
    schema?.example ??
    schema?.default ??
    schema?.enum?.[0] ??
    ''
  return value == null ? '' : String(value)
}

export function initialRequestValues(
  spec: OpenAPISpec,
  operation: ParsedOperation,
): RequestValues {
  const requestBody = getRequestBodySchema(spec, operation.requestBody)
  const bodyExample =
    requestBody?.example ?? schemaExample(spec, requestBody?.schema)
  return {
    parameters: Object.fromEntries(
      operation.parameters.map((parameter) => [
        `${parameter.in}:${parameter.name}`,
        initialParameterValue(spec, parameter),
      ]),
    ),
    body:
      bodyExample === undefined
        ? ''
        : JSON.stringify(bodyExample, null, 2),
    authSchemeKey: getSecuritySchemes(spec, operation)[0]?.key ?? '',
    authValue: '',
  }
}

export function getSecuritySchemes(
  spec: OpenAPISpec,
  operation: ParsedOperation,
): ResolvedSecurityScheme[] {
  if (
    operation.security.length === 0 ||
    operation.security.some((requirement) => Object.keys(requirement).length === 0)
  ) {
    return []
  }
  const requiredKeys = new Set(operation.security.flatMap(Object.keys))
  return Object.entries(spec.components?.securitySchemes ?? {}).flatMap(
    ([key, value]) => {
      if (requiredKeys.size > 0 && !requiredKeys.has(key)) return []
      const scheme = resolveReference<SecuritySchemeObject>(spec, value)
      return scheme ? [{ key, scheme }] : []
    },
  )
}

export function buildRequestURL(
  serverURL: string,
  operation: ParsedOperation,
  values: RequestValues,
): string {
  let path = operation.path
  const query = new URLSearchParams()

  operation.parameters.forEach((parameter) => {
    const value = values.parameters[`${parameter.in}:${parameter.name}`] ?? ''
    if (!value) return
    if (parameter.in === 'path') {
      path = path.replace(`{${parameter.name}}`, encodeURIComponent(value))
    }
    if (parameter.in === 'query') query.append(parameter.name, value)
  })

  const queryString = query.toString()
  return `${serverURL}${path.startsWith('/') ? path : `/${path}`}${
    queryString ? `?${queryString}` : ''
  }`
}

export function buildRequestHeaders(
  spec: OpenAPISpec,
  operation: ParsedOperation,
  values: RequestValues,
): Record<string, string> {
  const headers: Record<string, string> = {}
  operation.parameters.forEach((parameter) => {
    const value = values.parameters[`${parameter.in}:${parameter.name}`] ?? ''
    if (parameter.in === 'header' && value) headers[parameter.name] = value
  })

  const requestBody = getRequestBodySchema(spec, operation.requestBody)
  if (values.body && requestBody?.mediaType) {
    headers['Content-Type'] = requestBody.mediaType
  }

  const securitySchemes = getSecuritySchemes(spec, operation)
  const security =
    securitySchemes.find((item) => item.key === values.authSchemeKey) ??
    securitySchemes[0]
  if (security && values.authValue) {
    const scheme = security.scheme
    if (scheme.type === 'apiKey' && scheme.in === 'header' && scheme.name) {
      headers[scheme.name] = values.authValue
    } else if (scheme.type === 'http' && scheme.scheme === 'basic') {
      headers.Authorization = `Basic ${values.authValue}`
    } else {
      headers.Authorization = `Bearer ${values.authValue}`
    }
  }
  return headers
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`
}

export function buildCodeSample(
  language: CodeLanguage,
  spec: OpenAPISpec,
  operation: ParsedOperation,
  serverURL: string,
  values: RequestValues,
): string {
  const url = buildRequestURL(serverURL, operation, values)
  const headers = buildRequestHeaders(spec, operation, values)
  const method = operation.method.toUpperCase()
  const body = values.body.trim()

  if (language === 'javascript') {
    const init = [
      `method: '${method}'`,
      Object.keys(headers).length
        ? `headers: ${JSON.stringify(headers, null, 2)}`
        : '',
      body ? `body: JSON.stringify(${body})` : '',
    ].filter(Boolean)
    return `const response = await fetch('${url}', {\n  ${init.join(',\n  ')}\n})\n\nconst data = await response.json()`
  }

  if (language === 'python') {
    const args = [
      `'${url}'`,
      Object.keys(headers).length
        ? `headers=${JSON.stringify(headers, null, 2)}`
        : '',
      body ? `json=json.loads(${JSON.stringify(body)})` : '',
    ].filter(Boolean)
    const imports = body ? 'import json\nimport requests' : 'import requests'
    return `${imports}\n\nresponse = requests.${operation.method}(\n  ${args.join(',\n  ')},\n)\n\nprint(response.json())`
  }

  const lines = [`curl --request ${method} \\`, `  --url ${shellQuote(url)}`]
  Object.entries(headers).forEach(([name, value]) => {
    lines[lines.length - 1] += ' \\'
    lines.push(`  --header ${shellQuote(`${name}: ${value}`)}`)
  })
  if (body) {
    lines[lines.length - 1] += ' \\'
    lines.push(`  --data ${shellQuote(body)}`)
  }
  return lines.join('\n')
}

export function displaySchemaType(
  spec: OpenAPISpec,
  schemaLike: SchemaLike | undefined,
): string {
  if (!schemaLike) return 'any'
  if (isReference(schemaLike)) return schemaLike.$ref.split('/').at(-1) ?? 'object'
  const schema = resolveReference<SchemaObject>(spec, schemaLike)
  if (!schema) return 'any'
  if (schema.enum?.length) return schema.enum.map(String).join(' | ')
  if (schema.type === 'array') {
    return `array<${displaySchemaType(spec, schema.items)}>`
  }
  return schema.type || (schema.properties ? 'object' : 'any')
}

export function formatExample(value: unknown): string {
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

export function buildAIPrompt(
  spec: OpenAPISpec,
  operations: ParsedOperation[],
  referenceURL: string,
  apiVersion?: string,
): string {
  const title = spec.info?.title || 'this API'
  const servers = (spec.servers ?? [])
    .map(resolveServerURL)
    .filter(Boolean)
  const securitySchemes = Object.entries(
    spec.components?.securitySchemes ?? {},
  ).flatMap(([name, value]) => {
    const scheme = resolveReference<SecuritySchemeObject>(spec, value)
    if (!scheme) return []
    const kind =
      scheme.type === 'http'
        ? `${scheme.scheme || 'HTTP'}${scheme.bearerFormat ? ` (${scheme.bearerFormat})` : ''}`
        : scheme.type === 'apiKey'
          ? `API key in ${scheme.in || 'header'}${scheme.name ? ` named ${scheme.name}` : ''}`
          : scheme.type || 'configured security scheme'
    return [`- ${name}: ${kind}`]
  })
  const endpointCatalog = operations.map(
    (operation) =>
      `- ${operation.method.toUpperCase()} ${operation.path}  ${operation.summary}`,
  )

  return [
    `I need your help integrating the ${title}.`,
    '',
    `Authoritative API reference: ${referenceURL}`,
    `OpenAPI version: ${spec.openapi || 'not specified'}`,
    `API version: ${apiVersion || spec.info?.version || 'not specified'}`,
    '',
    'Base URLs:',
    ...(servers.length > 0 ? servers.map((server) => `- ${server}`) : ['- See the API reference']),
    '',
    'Authentication:',
    ...(securitySchemes.length > 0
      ? securitySchemes
      : ['- No global authentication scheme is documented']),
    '',
    `Available endpoints (${operations.length}):`,
    ...endpointCatalog,
    '',
    'Instructions:',
    '- Treat the linked Helpin API reference as the source of truth.',
    '- Read the documented parameters, request body, response schemas, and authentication requirements for every endpoint you use.',
    '- Do not invent fields or endpoints. Ask me for missing credentials, environment details, and the exact integration goal.',
    '- Produce secure, production-ready code with error handling and explain how to configure and test it.',
    '',
    'My integration goal:',
    '[Describe what you want the agent to build here]',
  ].join('\n')
}
