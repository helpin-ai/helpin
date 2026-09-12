---
name: api_docs_maintenance
description: Keeping existing API docs synchronized with shipped API behavior.
metadata:
  title: API Docs Maintenance
  supported_runtimes:
    - native_sdk
---

Use this skill when updating existing API documentation.

## Change Detection

- Check for changed endpoints, parameters, response fields, errors, auth, rate limits, pagination, and version notes.
- Confirm whether the change is additive, deprecated, breaking, or internal-only.
- Do not document behavior that is not shipped or explicitly approved for docs.

## Updates

- Use the workspace name from runtime context when a product, company, or workspace name is needed.
- Keep examples synchronized with the documented schema.
- Mark deprecations and breaking changes explicitly.
- Update SDK snippets, curl examples, field tables, enum values, and error examples together.
- Preserve compatibility notes when older clients may observe different behavior.

## Evidence

- Ground claims in route definitions, handlers, schemas, generated specs, tests, changelogs, or release artifacts.
- Call out missing implementation evidence before writing speculative API docs.
