# Verify signed-in widget visitors

Use this guide to connect your application's authenticated users to the Helpin
widget. Your backend signs the current user's identity; the browser sends that
identity and proof to Helpin. Anonymous pages can use the widget without a signed
identity, subject to the installation's origin policy.

The implemented contract uses an HMAC-SHA256 `identity_verification` object.
Earlier versions of this guide proposed a JWT with `sub` and `aud` claims; that
JWT format is not accepted by the current widget identity verifier.

## 1. Configure the installation

Add each website origin in Settings → Chat widget and copy the generated
installation snippet. Origins include the scheme and any non-default port, with
no path or wildcard. An empty allowlist denies widget access. Use the SDK/API
URLs belonging to your Helpin deployment; the current loader filename is `lib.js`.

Obtain the installation's signing secret through the authenticated secret-rotation
endpoint described in [website origins and visitor identity](community/widget-identity.md).
Keep it only in your application's backend secret configuration. The public widget
key is a different value. Rotation invalidates proofs signed with the previous key.

## 2. Sign the authenticated user's identity

Implement an endpoint in your application that requires your normal authenticated
session. Derive the user ID, email, and company ID from that session and authorized
account data. Do not accept arbitrary identities from the browser for signing.
Return the identity fields together with a fresh proof, and prevent shared caches
from storing the response.

Use the [canonical payload and Python example](community/widget-identity.md#signed-identities)
to construct the proof. In order, the signed newline-separated values are:

1. `helpin-widget-identity:v1`
2. The installation's widget key
3. The email, trimmed and lowercased
4. The external user ID, exactly as submitted
5. The company ID, trimmed, or an empty string
6. The issued-at Unix timestamp in seconds
7. The expires-at Unix timestamp in seconds

Sign those bytes with HMAC-SHA256, using the secret's UTF-8 bytes without
hex-decoding it. Return the signature as hexadecimal in this structure:

```json
{
  "version": "v1",
  "issued_at": 1234567890,
  "expires_at": 1234568190,
  "signature": "HEX_HMAC_SHA256_SIGNATURE"
}
```

The timestamps above illustrate field types and are expired examples. Generate
fresh timestamps: validity must be positive and no longer than 15 minutes;
issue time may be at most one minute ahead of the server clock. A five-minute
proof is sufficient for the initial identification request. Send the same identity
values that were signed.

## 3. Identify through the SDK

After loading the SDK and initializing a client, fetch the signed identity from
your own backend and pass it to `id(...)`. This example assumes your endpoint
returns `{ id, email, company, identity_verification }` using SDK field names:

```ts
import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'YOUR_WIDGET_KEY',
  host: 'https://widget.example.com',
  widgetRuntimeUrl: 'https://widget.example.com/sdk/lib.js',
  autoBoot: false,
});
if (!client) throw new Error('Helpin client failed to initialize');

const response = await fetch('/api/helpin/identity', {
  credentials: 'same-origin',
  cache: 'no-store',
});
if (!response.ok) throw new Error('Could not load signed widget identity');
const identity = await response.json();
await client.id(identity);
client.open();
```

`/api/helpin/identity` is an example endpoint you implement, not a built-in Helpin
route. Configure the host and runtime URL for your deployment. See the
[SDK README](../packages/sdk-js/README.md) for module and script-tag installation.

## 4. Enable enforcement and verify

Select **Require server-signed identities** after integrating and checking the
signer. In `enforced` mode, missing, invalid, expired, or mismatched proofs reject
identity requests. In `report_only`, those requests remain unverified browser
claims; a valid proof can establish verified identity. In `off`, the verifier
returns untrusted provenance. These modes do not replace the origin allowlist.

New Community installations default to `report_only`; new Cloud and Enterprise installations
use `enforced`. Existing installations retain their saved mode. Check that mode rather
than assuming unsigned identities are always rejected.

Test with a valid identity, an expired proof, and a changed email or company ID.
Also test anonymous chat and a disallowed website origin. A successful script
load alone does not prove that identity verification succeeded. This guide's
source review did not execute your application's integration.

## Diagnose failures

- Check that the signer uses the installation's signing secret, not its public key.
- Compare signed and submitted identity values, including user ID and company ID.
- Check Unix seconds, proof lifetime, clock skew, and whether the secret was rotated.
- Inspect origin and identity errors separately; a valid proof does not authorize
  a disallowed embedding origin.

The authoritative implementation is the
[widget identity verifier](../server/internal/service/support_widget_identity.go),
with [verification tests](../server/internal/service/support_widget_identity_test.go).
