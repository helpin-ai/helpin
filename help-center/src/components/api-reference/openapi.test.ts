import { describe, expect, it } from 'vitest'
import {
  buildCodeSample,
  buildAIPrompt,
  buildRequestHeaders,
  buildRequestURL,
  collectOperations,
  getSecuritySchemes,
  initialRequestValues,
  schemaExample,
  type OpenAPISpec,
} from './openapi'

const spec: OpenAPISpec = {
  openapi: '3.1.0',
  info: { title: 'Example API', version: '1.0.0' },
  servers: [{ url: 'https://api.example.com' }],
  security: [{ bearerAuth: [] }],
  paths: {
    '/projects/{projectId}': {
      parameters: [
        {
          name: 'projectId',
          in: 'path',
          required: true,
          schema: { type: 'string', format: 'uuid' },
        },
      ],
      post: {
        operationId: 'updateProject',
        summary: 'Update a project',
        tags: ['Projects'],
        parameters: [
          {
            name: 'include',
            in: 'query',
            schema: { type: 'string', default: 'members' },
          },
        ],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: { $ref: '#/components/schemas/ProjectInput' },
            },
          },
        },
        responses: {
          200: {
            description: 'Updated',
            content: {
              'application/json': {
                schema: { $ref: '#/components/schemas/Project' },
              },
            },
          },
        },
      },
    },
  },
  components: {
    securitySchemes: {
      bearerAuth: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT' },
    },
    schemas: {
      ProjectInput: {
        type: 'object',
        required: ['name'],
        properties: {
          name: { type: 'string', example: 'Website redesign' },
          active: { type: 'boolean', default: true },
        },
      },
      Project: {
        allOf: [
          { $ref: '#/components/schemas/ProjectInput' },
          {
            type: 'object',
            properties: { id: { type: 'string', format: 'uuid' } },
          },
        ],
      },
    },
  },
}

describe('native OpenAPI renderer helpers', () => {
  it('collects operations with shared and operation parameters', () => {
    const [operation] = collectOperations(spec)

    expect(operation).toMatchObject({
      id: 'updateProject',
      method: 'post',
      path: '/projects/{projectId}',
      tags: ['Projects'],
    })
    expect(operation?.parameters.map((parameter) => parameter.name)).toEqual([
      'projectId',
      'include',
    ])
  })

  it('builds useful initial parameter and request-body values', () => {
    const operation = collectOperations(spec)[0]!
    const values = initialRequestValues(spec, operation)

    expect(values.parameters['path:projectId']).toBe('')
    expect(values.parameters['query:include']).toBe('members')
    expect(JSON.parse(values.body)).toEqual({
      name: 'Website redesign',
      active: true,
    })
  })

  it('resolves composed schema examples', () => {
    expect(schemaExample(spec, { $ref: '#/components/schemas/Project' })).toEqual({
      name: 'Website redesign',
      active: true,
      id: '00000000-0000-4000-8000-000000000000',
    })
  })

  it('builds the request URL, headers, and auth from current inputs', () => {
    const operation = collectOperations(spec)[0]!
    const values = initialRequestValues(spec, operation)
    values.parameters['path:projectId'] = 'project 1'
    values.authValue = 'secret'

    expect(
      buildRequestURL('https://api.example.com', operation, values),
    ).toBe('https://api.example.com/projects/project%201?include=members')
    expect(buildRequestHeaders(spec, operation, values)).toEqual({
      'Content-Type': 'application/json',
      Authorization: 'Bearer secret',
    })
    expect(getSecuritySchemes(spec, operation)[0]?.key).toBe('bearerAuth')
  })

  it('generates cURL, JavaScript, and Python samples', () => {
    const operation = collectOperations(spec)[0]!
    const values = initialRequestValues(spec, operation)
    values.parameters['path:projectId'] = '123'

    expect(
      buildCodeSample(
        'curl',
        spec,
        operation,
        'https://api.example.com',
        values,
      ),
    ).toContain('curl --request POST')
    expect(
      buildCodeSample(
        'javascript',
        spec,
        operation,
        'https://api.example.com',
        values,
      ),
    ).toContain('await fetch')
    expect(
      buildCodeSample(
        'python',
        spec,
        operation,
        'https://api.example.com',
        values,
      ),
    ).toContain('json=json.loads')
  })

  it('does not request credentials for explicitly public operations', () => {
    const publicSpec: OpenAPISpec = {
      ...spec,
      paths: {
        '/health': {
          get: {
            summary: 'Health check',
            security: [],
            responses: { 200: { description: 'Healthy' } },
          },
        },
      },
    }
    const operation = collectOperations(publicSpec)[0]!

    expect(getSecuritySchemes(publicSpec, operation)).toEqual([])
  })

  it('uses the selected authentication alternative', () => {
    const alternativeSpec: OpenAPISpec = {
      ...spec,
      security: [{ bearerAuth: [] }, { apiKey: [] }],
      components: {
        ...spec.components,
        securitySchemes: {
          ...spec.components?.securitySchemes,
          apiKey: { type: 'apiKey', in: 'header', name: 'X-API-KEY' },
        },
      },
    }
    const operation = collectOperations(alternativeSpec)[0]!
    const values = initialRequestValues(alternativeSpec, operation)
    values.authSchemeKey = 'apiKey'
    values.authValue = 'api-secret'

    expect(getSecuritySchemes(alternativeSpec, operation).map(({ key }) => key)).toEqual([
      'bearerAuth',
      'apiKey',
    ])
    expect(buildRequestHeaders(alternativeSpec, operation, values)).toMatchObject({
      'X-API-KEY': 'api-secret',
    })
    expect(buildRequestHeaders(alternativeSpec, operation, values)).not.toHaveProperty(
      'Authorization',
    )
  })

  it('builds a ready-to-paste agent prompt from the reference', () => {
    const prompt = buildAIPrompt(
      spec,
      collectOperations(spec),
      'https://docs.example.com/api/reference',
      '1.0.0',
    )

    expect(prompt).toContain(
      'Authoritative API reference: https://docs.example.com/api/reference',
    )
    expect(prompt).toContain(
      '- POST /projects/{projectId}  Update a project',
    )
    expect(prompt).toContain('- bearerAuth: bearer (JWT)')
    expect(prompt).toContain('[Describe what you want the agent to build here]')
  })
})
