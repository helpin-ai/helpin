# Send lead events to Helpin

Learn how to send first-party lead data to Helpin using the JavaScript SDK’s `lead` API. Lead events capture contact details for prospects that have not been fully identified as users yet.

## When to Use Lead Events

- Capture form submissions where you only have marketing-qualified contact details.
- Forward leads collected from partner integrations, landing pages, or CRM imports.
- Trigger downstream automations that depend on valid contact information.

## Prerequisites

- Install and initialize the Helpin JavaScript SDK (see the getting started guide for credentials and setup).
- Ensure each lead payload includes a valid `email` field. Events without a properly formatted email are ignored and an error is logged to the console.

## Basic Usage

```ts
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'UM_PUBLIC_KEY',
  host: 'https://events.helpin.ai',
});

client?.lead({
  email: 'prospect@example.com',
  first_name: 'Jamie',
  last_name: 'Rivera',
  company: {
    id: 'company_123',
    name: 'Acme Corp',
    created_at: '2024-01-15T00:00:00Z',
  },
  phone: '+1 555 0100',
  lifecycle_stage: 'marketing_qualified',
});
```

### Command-style API

Load the SDK script with your widget key as described in the [SDK README](../README.md).
This queue stub buffers calls until the script initializes; it does not load the SDK.

```html
<script>
  window.helpin = window.helpin || function () {
    (window.helpinQ = window.helpinQ || []).push(Array.from(arguments));
  };
  window.helpin('lead', {
    email: 'prospect@example.com',
    first_name: 'Jamie',
    last_name: 'Rivera',
    company: {
      id: 'company_123',
      name: 'Acme Corp',
      created_at: '2024-01-15T00:00:00Z'
    },
    source: 'Webinar Sign-up',
    campaign: 'Q1 Product Launch'
  });
</script>
```

## Payload Requirements

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `email` | string | ✔︎ | Must be non-empty and pass standard email validation. Trimmed automatically before sending. |
| `...` | any | optional | Add any custom attributes (e.g., `first_name`, `last_name`, `company`, `utm_source`, `plan_interest`). |

A null, array, or other non-object payload throws an error. An object with a
missing or invalid email logs `Lead event requires a valid email attribute` and
returns without sending. Valid emails are trimmed on the supplied object.

The method sends a `lead` analytics event and separately attempts backend identity
synchronization for CRM lead creation and conversation backfill. In support-only
mode analytics tracking is skipped; backend identity synchronization still runs.
See the [implementation](../src/core/client.ts).

## Direct Send vs. Queued Delivery

By default, lead events are queued and retried like other tracked events. Pass `true` as the second argument to use direct analytics delivery. This is not a guarantee of delivery during page unload, and it does not change the separate backend identity request:

```ts
client?.lead({ email: 'fastsend@example.com' }, true);
```

## Framework Integrations

- **Next.js**: `const { lead } = useHelpin(); lead({ email: 'lead@example.com' });`
- **React**: `const { lead } = useHelpin(); lead({ email: 'lead@example.com', source: 'Adwords' });`
- **Vue**: After installing `HelpinPlugin`, use `const helpin = useHelpin(); helpin.lead({ email: 'lead@example.com' });`. See the [Vue guide](../../vue/README.md) for Nuxt setup.

These helpers forward calls to the core `lead` API and inherit the same validation rules.

## Best Practices

- Always collect explicit consent before sending lead data where required by law.
- Populate contextual attributes (campaign, touchpoint, lifecycle stage) to improve segmentation and reporting.
- Use consistent casing for custom keys to simplify downstream analytics.
- Combine with `set` or `group` calls when a lead transitions into a fully identified user or company.

## Troubleshooting

- Check the browser console for the validation error if events are not recorded.
- Confirm the project key and tracking host are correct and that ad blockers aren’t preventing requests.
- If leads are collected server-side, ensure the environment can reach `host` and forward the same payload structure.
