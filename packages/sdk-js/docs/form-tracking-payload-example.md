# Form capture payload

Use this reference when configuring SDK form capture or consuming its `$form`
events. Capture is opt-in: only forms matching a configured selector are observed,
and only explicitly allowed field names are collected.

## Configure a form

Pass `formCapture` when initializing the client:

```ts
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'YOUR_WIDGET_KEY',
  formCapture: [{
    formId: 'contact-sales',
    selector: '#contact-sales',
    fields: ['email', 'company_name'],
    fieldMappings: {
      email: 'contact.email',
      company_name: 'company.name',
    },
  }],
});
```

## Event attributes

For a matching form submission, the SDK calls `track('$form', attributes)` with
this shape. The tracking transport supplies the surrounding event envelope.

```json
{
  "form_id": "contact-sales",
  "field_names": ["email", "company_name"],
  "fields": {
    "email": "prospect@example.com",
    "company_name": "Example Company"
  },
  "field_mappings": {
    "email": "contact.email",
    "company_name": "company.name"
  }
}
```

`fields` is an object keyed by field name, not the array of DOM element metadata
shown in older examples. Values are limited to 256 characters. Password and file
inputs, non-string form values, and the implementation's reserved sensitive field
names are excluded. An empty allowlist captures no field values. Configure the
allowlist deliberately; these exclusions do not classify every possible sensitive
field automatically.

Recognized field names and input types can supply automatic mappings when an
explicit mapping is absent. A mapped email can also produce a `lead` call before
the `$form` event. The lead API performs its own email validation.

## Removed legacy behavior

The current capture implementation does not install a field-change listener or
emit `$form_field_change`. Earlier examples of that event and full DOM attributes
are historical and should not be used as the current payload contract.

See [capture implementation](../src/tracking/configured-capture.ts),
[configuration types](../src/core/types.ts), and
[lead events](sending-lead-events.md).
