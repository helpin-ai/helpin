# Guarded CRM Enrichment Tools Plan

**Date:** 2026-04-29
**Status:** Implemented
**Scope:** Add CRM contact/company mutation tools for one-shot Command Agent runs, with server-side guards that prevent agents from rewriting user-entered identity data.

---

## Problem

One-shot Command Agent runs can now research CRM contacts and companies, but they cannot safely write the results back. The existing broad CRM update services allow fields like contact names, email, phone, and company name to be changed. That is too much authority for an LLM-facing enrichment tool.

The first tool slice should support "find info about this contact and update contact and company" while enforcing a conservative rule:

- names cannot be changed by agent tools
- existing email and phone cannot be changed
- existing company name and domain cannot be changed
- agent-discovered data can only fill empty allowed fields or append agent-owned enrichment metadata
- every applied or skipped field change is returned in structured JSON
- every attempt is recorded as CRM enrichment evidence

Important constraint: the current CRM models do not track per-field provenance. Until field provenance exists, the backend cannot reliably know whether a non-empty value was user-entered, imported, or previously agent-written. Therefore v1 treats every non-empty protected value as user-owned and refuses to overwrite it.

## Current Model Grounding

Contact fields:

- `first_name`, `last_name`: existing mutable service fields, but not exposed to the agent enrichment tool.
- `email`, `phone`: fill-only. If non-empty, skip with `existing_value_protected`.
- `job_title`, `avatar_url`: fill-only in v1.
- `lifecycle_stage`, `lead_status`, `owner_member_id`, `source`: not exposed in this tool.
- `custom_properties`: only agent-owned keys under an enrichment namespace.

Company fields:

- `name`: not exposed to the agent enrichment tool.
- `domain`: fill-only. If non-empty, skip with `existing_value_protected`.
- `industry`, `employee_count`, `annual_revenue`, `description`, `logo_url`: fill-only in v1.
- `owner_member_id`, `external_id`: not exposed in this tool.
- `custom_properties`: only agent-owned keys under an enrichment namespace.

Existing audit surface:

- `crm_enrichment_results` already stores `workspace_id`, `object_type`, `object_id`, `source`, `data`, `confidence`, and `created_at`.
- Reuse this table for raw evidence and the applied/skipped ledger. Do not add a new audit table in the first slice unless the output shape outgrows `data`.

## Tool Contract

Add two command-backed runtime tools:

- `ensure_crm_contact_company`
- `enrich_crm_contact`
- `enrich_crm_company`

These are product mutation tools, so the model-facing tool contract is backed by internal commands:

| Runtime tool | Internal command |
|---|---|
| `ensure_crm_contact_company` | `crm.ensure_contact_company` |
| `enrich_crm_contact` | `crm.enrich_contact` |
| `enrich_crm_company` | `crm.enrich_company` |

### Shared Input Shape

Use a field-list shape instead of an arbitrary patch object. That makes the guard explicit and keeps unknown fields rejectable.

```json
{
  "contact_id": "uuid",
  "fields": [
    {
      "field": "job_title",
      "value": "VP of Sales",
      "source_url": "https://example.com/team",
      "evidence": "Public profile lists this title.",
      "confidence": 0.86
    }
  ],
  "evidence_summary": "Found public role and company details from source pages.",
  "dry_run": false
}
```

For company:

```json
{
  "company_id": "uuid",
  "fields": [
    {
      "field": "industry",
      "value": "Product Analytics",
      "source_url": "https://example.com/about",
      "evidence": "About page describes the company as a product analytics platform.",
      "confidence": 0.82
    }
  ],
  "evidence_summary": "Found industry and company summary from public website.",
  "dry_run": false
}
```

Schema rules:

- top-level object
- explicit properties
- `additionalProperties: false`
- required: object ID and `fields`
- `fields` item required: `field`, `value`, `source_url`, `evidence`, `confidence`
- `confidence` range: `0.0` to `1.0`
- max `fields`: 20
- max `value`: 500 chars, except company `description` max 2,000 chars
- max `evidence`: 1,000 chars
- `dry_run` defaults to false

### Contact Company Ensure Input Shape

This tool covers the common contact-page case where the agent has evidence for the contact's company, but no company record is associated yet.

```json
{
  "contact_id": "uuid",
  "company_name": "Usermaven",
  "domain": "usermaven.com",
  "source_url": "https://usermaven.com",
  "evidence": "The contact email domain and public company website identify Usermaven.",
  "confidence": 0.95,
  "association_label": "primary",
  "dry_run": false
}
```

Rules:

- matches an existing company by domain first, then name
- creates a company only when no match exists
- does not modify existing company `name` or `domain`
- links the contact to the company with `primary` by default
- returns whether the company/link was created or reused
- records the action in `crm_enrichment_results`

### Contact Field Enum

Allowed in v1:

- `email`
- `phone`
- `job_title`
- `avatar_url`
- `linkedin_url`
- `location`
- `enrichment_note`

Not allowed:

- `first_name`
- `last_name`
- `display_id`
- `lifecycle_stage`
- `lead_status`
- `owner_member_id`
- `source`
- arbitrary `custom_properties`

Field mapping:

| Tool field | Storage |
|---|---|
| `email` | `crm_contacts.email`, fill-only |
| `phone` | `crm_contacts.phone`, fill-only |
| `job_title` | `crm_contacts.job_title`, fill-only |
| `avatar_url` | `crm_contacts.avatar_url`, fill-only |
| `linkedin_url` | `custom_properties.agent_enrichment.linkedin_url`, append/upsert agent-owned key |
| `location` | `custom_properties.agent_enrichment.location`, append/upsert agent-owned key |
| `enrichment_note` | `custom_properties.agent_enrichment.notes[]`, append-only |

### Company Field Enum

Allowed in v1:

- `domain`
- `industry`
- `employee_count`
- `annual_revenue`
- `description`
- `logo_url`
- `linkedin_url`
- `headquarters`
- `enrichment_note`

Not allowed:

- `name`
- `display_id`
- `external_id`
- `owner_member_id`
- arbitrary `custom_properties`

Field mapping:

| Tool field | Storage |
|---|---|
| `domain` | `crm_companies.domain`, fill-only |
| `industry` | `crm_companies.industry`, fill-only |
| `employee_count` | `crm_companies.employee_count`, fill-only |
| `annual_revenue` | `crm_companies.annual_revenue`, fill-only |
| `description` | `crm_companies.description`, fill-only |
| `logo_url` | `crm_companies.logo_url`, fill-only |
| `linkedin_url` | `custom_properties.agent_enrichment.linkedin_url`, append/upsert agent-owned key |
| `headquarters` | `custom_properties.agent_enrichment.headquarters`, append/upsert agent-owned key |
| `enrichment_note` | `custom_properties.agent_enrichment.notes[]`, append-only |

## Guard Policy

The service, not the prompt, owns the safety guarantees.

### Hard Reject

Reject the tool call before writing when:

- target ID is missing
- target is outside the run workspace
- field enum contains a disallowed field
- field value is empty after trimming
- `source_url` is empty or invalid
- `confidence` is below `0.70`
- more than 20 fields are requested
- scalar type is invalid, such as non-integer `employee_count`

### Skip But Succeed

Do not fail the whole call when one field is unsafe. Skip that field and return a structured reason:

- `existing_value_protected`: current field is non-empty and different
- `same_value`: current field already matches the proposed value
- `invalid_value`: field-specific validation failed after parsing
- `low_confidence`: confidence below threshold if the implementation chooses per-field skip instead of hard reject

### Fill-Only Rule

For core CRM columns, v1 writes only when the current value is empty. This includes contact `email`, `phone`, and `job_title`; company `domain`, `industry`, `employee_count`, `annual_revenue`, `description`, and `logo_url`.

No agent tool may change contact names or company names in v1.

### Agent-Owned Metadata Rule

Only `custom_properties.agent_enrichment` can be written by these tools. The tool must never replace the entire `custom_properties` object and must never write arbitrary top-level custom-property keys.

Recommended shape:

```json
{
  "agent_enrichment": {
    "linkedin_url": {
      "value": "https://www.linkedin.com/in/example",
      "source_url": "https://www.linkedin.com/in/example",
      "confidence": 0.88,
      "updated_at": "2026-04-29T00:00:00Z"
    },
    "notes": [
      {
        "value": "Likely involved in analytics buying decision.",
        "source_url": "https://example.com/interview",
        "confidence": 0.76,
        "created_at": "2026-04-29T00:00:00Z"
      }
    ]
  }
}
```

## Output Shape

Return compact JSON:

```json
{
  "status": "partial",
  "object_type": "contact",
  "object_id": "uuid",
  "applied": [
    {
      "field": "job_title",
      "old_value": null,
      "new_value": "VP of Sales",
      "source_url": "https://example.com/team",
      "confidence": 0.86
    }
  ],
  "skipped": [
    {
      "field": "email",
      "reason": "existing_value_protected",
      "current_value_present": true,
      "proposed_value": "new@example.com"
    }
  ],
  "enrichment_result_id": "uuid",
  "dry_run": false
}
```

Status values:

- `applied`: at least one field applied and none skipped
- `partial`: at least one field applied and at least one skipped
- `skipped`: no fields applied, at least one skipped
- `dry_run`: no fields written

## Implementation Work

### Backend

1. [x] Add request/response DTOs for guarded enrichment in `server/internal/model`.
2. [x] Extend `CRMEnrichmentService` with:
   - contact guarded apply
   - company guarded apply
   - field normalization
   - workspace ownership checks
   - `crm_enrichment_results` audit write
3. [x] Add internal commands:
   - `crm.ensure_contact_company`
   - `crm.enrich_contact`
   - `crm.enrich_company`
4. [x] Add runtime tools in `server/internal/worker/tools.go` and `server/internal/worker/tools_crm.go`.
5. [x] Add metadata in `server/internal/commandtools/metadata.go` and catalog category mapping.
6. [x] Expose tools only to the product-owned Command Agent first. Do not add to CRM Operator until we have observed enough one-shot runs.
7. [x] Update command-bar one-shot tool selection:
   - CRM research/update prompts include `enrich_crm_contact`
   - contact page prompts that mention company include `ensure_crm_contact_company` and `enrich_crm_company`
   - keep `request_approval` in the proposed tool set for broad CRM update prompts
8. [x] Update one-shot execution brief to say protected fields are enforced by tools, not just by instruction.

### Frontend

No new frontend surface is required for the first backend slice. The existing one-shot confirmation UI should show these tools in the proposed tool list.

After backend lands, add a small warning under CRM one-shot plans:

> Protected CRM fields cannot be overwritten. Names, existing email, existing phone, existing company name, and existing domain are skipped by the tool.

## Test Plan

Backend unit tests:

- [x] `enrich_crm_contact` rejects `first_name` / `last_name`.
- [x] `enrich_crm_company` rejects `name`.
- [x] `ensure_crm_contact_company` creates and associates a missing company.
- [x] `ensure_crm_contact_company` reuses an existing company by domain without changing existing identity fields.
- [x] non-empty contact `email` is skipped, not overwritten.
- [x] empty contact `phone` and `job_title` are filled when source/confidence are valid.
- [x] non-empty company `domain` is skipped, not overwritten.
- [x] empty company `industry` is filled when source/confidence are valid.
- [x] `custom_properties.agent_enrichment` merge preserves existing unrelated custom properties.
- [x] every successful call creates a `crm_enrichment_results` row with applied/skipped details.
- [x] dry run returns the same applied/skipped plan without writing columns.
- [x] wrong-workspace target is rejected by an explicit test.
- [x] command-bar CRM one-shot parser includes the new tools for "find info about this contact and update contact and company".

Worker/tool contract tests:

- [x] tool schemas include `additionalProperties: false`.
- [x] tool schemas require object ID and fields.
- [x] tool catalog categorizes the contact enrichment tool as CRM.
- [x] Command Agent allowlist includes both tools; CRM Operator allowlist does not in this first slice.

## Non-Goals

- Changing contact or company names.
- Replacing existing user-entered email, phone, domain, or company identity fields.
- Arbitrary custom-property writes.
- Full field-level provenance tracking.
- Structured CRM diff approval UI.
- Giving CRM Operator these tools by default.

## Future Follow-Ups

- Add field-level provenance so previously agent-written fields can be refreshed while user-owned fields stay protected.
- Add a structured proposal entity for review-before-apply on non-fill-only changes.
- Add association tools, such as linking a contact to a company, after the protected-field write path is proven.
- Consider adding the guarded tools to CRM Operator once command-run telemetry shows stable behavior.
