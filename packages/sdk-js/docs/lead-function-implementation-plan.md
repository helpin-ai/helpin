# Lead function implementation plan (historical)

> Historical implementation plan. `lead` is implemented in
> [`src/core/client.ts`](../src/core/client.ts); the old `packages/javascript-sdk`
> paths below predate the package move. Use the [SDK reference](../README.md#client-api)
> for the current API, including CRM identity synchronization.


## Objective

Introduce a dedicated `lead` API in the JavaScript SDK so that first-party applications can send lead events that always include a contact email before the payload is dispatched to Helpin.

## Implementation Overview

1. Extend `HelpinClient` with a `lead` method that mirrors `track` while enforcing the presence of a valid `email` field in `event_attributes`.
2. Update public SDK types and command-style facades so that `lead` is available across object-oriented and global invocation patterns.
3. Maintain existing retry, transport, and payload-building flows by reusing `trackInternal` once validation passes.

## Detailed Steps

1. **Core client (`packages/javascript-sdk/src/core/client.ts`)**
   - Add `public lead(payload: EventPayload, directSend: boolean = false): void`.
   - Validate the payload is a non-null object and not an array; throw `Lead payload must be a non-null object and not an array` when invalid.
   - Require `payload.email` to be a non-empty string that satisfies `isValidEmail`.
   - If the email check fails, log `this.logger.error('Lead event requires a valid email attribute');` and return without invoking the retry queue or transport.
   - On success, call `this.track('lead', payload, directSend);` to reuse existing queuing and delivery logic.
2. **Public surface area**
   - Update `HelpinGlobal` in `packages/javascript-sdk/src/core/types.ts` to include command-style and object-oriented overloads for `lead`.
   - Ensure any helper types or exports that mirror client methods (e.g., type definitions re-exported from the package) include the new API.
3. **Command queue bridge (`packages/javascript-sdk/src/index.ts`)**
   - Add `'lead'` to the synchronous methods list so queued script-tag commands flush correctly.
   - Confirm the namespaced function proxies `lead` calls to the instantiated client.
4. **Wrapper checks**
   - Verify framework-specific wrappers (Next.js hook, Nuxt plugin, etc.) already spread the underlying client; no changes required beyond type augmentation. If a wrapper enumerates exposed methods, add `lead` there as needed.

## Validation Rules

- Payload must be an object (not `null` or an array); otherwise throw.
- `payload.email` must exist, be a trimmed non-empty string, and match the existing `isValidEmail` helper.
- When validation fails, emit a single logger error and exit early without enqueuing or sending the event.
- Respect the `directSend` flag by passing it through to `track`.

## Testing Strategy

Add unit coverage to `packages/javascript-sdk/test/unit/core/client.test.ts`:

1. **Happy path** – Given a payload with a valid email, `client.lead(payload)` should call `client.track('lead', payload)` and not log errors.
2. **Missing email** – When `payload.email` is absent or empty, `client.lead(payload)` should log an error and never invoke `client.track` or the retry queue.
3. **Invalid email format** – For an incorrectly formatted email, expect the same error logging/skip behavior as the missing case.
4. **Direct send flag** – `client.lead(payload, true)` should call `client.track('lead', payload, true)` to ensure immediate transport works.
5. **Invalid payload type** – Passing a non-object (e.g., string or array) should throw with the new validation error message.

## Rollout Considerations

- No configuration changes are required; client initialization remains unchanged.
- Documentation updates should highlight the new API once implementation lands.
- Downstream applications should be advised that the lead payload must always include an `email` field.
