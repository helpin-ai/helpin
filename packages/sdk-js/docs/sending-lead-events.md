# Sending Lead Events to Helpin

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
import { helpinClient } from '@helpin/sdk-js';

const client = helpinClient({
  key: 'UM_PUBLIC_KEY',
  trackingHost: 'https://events.helpin.ai',
});

client.lead({
  email: 'prospect@example.com',
  first_name: 'Jamie',
  last_name: 'Rivera',
  company: 'Acme Corp',
  phone: '+1 555 0100',
  lifecycle_stage: 'marketing_qualified',
});
```

### Command-Style API

```html
<script>
  window.helpin = window.helpin || [];
  window.helpin('lead', {
    email: 'prospect@example.com',
    source: 'Webinar Sign-up',
    campaign: 'Q1 Product Launch'
  });
</script>
```

## Payload Requirements

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `email` | string | ✔︎ | Must be non-empty and pass standard email validation. Trimmed automatically before sending. |
| `...` | any | optional | Add any custom attributes (e.g., `first_name`, `utm_source`, `plan_interest`). |

If the payload is not an object or the `email` field is missing/invalid, the SDK logs `Lead event requires a valid email attribute` and skips sending the event.

## Direct Send vs. Queued Delivery

By default, lead events are queued and retried like other tracked events. To bypass the retry queue (for example, immediately after a form submission on page unload), pass `true` as the second argument:

```ts
client.lead({ email: 'fastsend@example.com' }, true);
```

## Framework Integrations

- **Next.js**: `const { lead } = useHelpin(); lead({ email: 'lead@example.com' });`
- **React**: `const { lead } = useHelpin(); lead({ email: 'lead@example.com', source: 'Adwords' });`
- **Nuxt/Vue**: Use the injected `$helpin` client: `await $helpin.lead({ email: 'lead@example.com' });`

These helpers forward calls to the core `lead` API and inherit the same validation rules.

## Best Practices

- Always collect explicit consent before sending lead data where required by law.
- Populate contextual attributes (campaign, touchpoint, lifecycle stage) to improve segmentation and reporting.
- Use consistent casing for custom keys to simplify downstream analytics.
- Combine with `set` or `group` calls when a lead transitions into a fully identified user or company.

## Troubleshooting

- Check the browser console for the validation error if events are not recorded.
- Confirm the project key and tracking host are correct and that ad blockers aren’t preventing requests.
- If leads are collected server-side, ensure the environment can reach `trackingHost` and forward the same payload structure.
