---
name: api_doc_writing
description: Writing new API reference docs and API guides.
metadata:
  title: API Doc Writing
  supported_runtimes:
    - native_sdk
---

Use this skill when creating new API documentation.

## Accuracy

- Do not invent endpoints, fields, limits, or SDK behavior.
- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Ground API docs in source code, schemas, route definitions, generated specs, or explicitly provided product facts.
- Call out uncertainty when the implementation or contract is not available.

## Required Coverage

- Document authentication, permissions, request shape, response shape, errors, and examples.
- Include path parameters, query parameters, headers, body fields, enum values, pagination, rate limits, idempotency, and webhooks when applicable.
- Include at least one realistic request example and one realistic response example when the endpoint has a request or response body.
- Show error examples for common failure cases when known.

## Style

- Use precise field names and stable casing.
- Keep examples valid JSON or valid code for the stated language.
- Separate conceptual API guides from endpoint reference pages.
- Mention versioning and compatibility when behavior differs by version or release.
