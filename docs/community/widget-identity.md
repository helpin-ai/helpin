# Website origins and visitor identity

This guide is for operators and website developers. It explains how website
origins are allowlisted and how visitor identity is verified; apply it before
embedding the widget on a site.

In Settings → Chat widget, first add your website origin, for example
`https://www.example.com`. Use one origin per line, with no path, trailing slash,
or wildcard. Add preview and local development origins explicitly, including
non-default ports. Copy the installation snippet after saving. An empty list
denies all widget access, including existing session tokens. Origin enforcement
applies to HTTP and both WebSocket handshakes independently of identity mode.
Static SDK assets and published help-center articles remain public.

New Community installations accept unsigned identities as **unverified visitor
claims** (`report_only`). Matching another visitor's email does not authorize
access to that visitor's conversations. An origin allowlist limits browser
embedding; it is not authentication for non-browser clients.

Enterprise keeps `enforced` for new installations. Both lazy creation and
workspace seeding write the edition mode explicitly. Existing installations
retain their saved mode. The column default in the current schema is
`report_only`; no upgrade migration changes an existing installation's saved
mode. Use the application setup path instead of inserting installations directly
into the database.

## Signed identities

Select **Require server-signed identities** only after integrating signing on
your application's server. Anonymous chat remains available. Identity requests
with missing, expired, or mismatched signatures are rejected in this mode.

An authorized workspace administrator can obtain a new signing secret using
`POST /api/support/inbox/installations/rotate-secret?workspace_id=WORKSPACE_ID`
with their authenticated session. The response contains `secret_key` once. Store
it in your backend's secret configuration; never in the browser or an install
snippet. Rotation immediately invalidates proofs signed with the previous key.
The public widget key and server signing secret are different values.

Example server-side signing in Python (standard library only):

```python
import hashlib
import hmac
import time

def sign_identity(secret, widget_key, email, external_user_id="", company_id=""):
    issued_at = int(time.time())
    expires_at = issued_at + 300
    payload = "\n".join([
        "helpin-widget-identity:v1",
        widget_key,
        email.strip().lower(),
        external_user_id,
        company_id.strip(),
        str(issued_at),
        str(expires_at),
    ])
    return {
        "version": "v1",
        "issued_at": issued_at,
        "expires_at": expires_at,
        "signature": hmac.new(secret.encode(), payload.encode(), hashlib.sha256).hexdigest(),
    }
```

Attach the result as `identity_verification` to the identity payload. Sign the
same email, external user ID and company ID you submit. The secret is used as
its UTF-8 string, not hex-decoded. The maximum validity is 15 minutes; issue times
allow at most one minute of forward clock skew. Use HTTPS and keep clocks in sync.
Do not provide a signing endpoint that signs arbitrary visitor-supplied identities:
derive them from the logged-in user on your own backend.

Browsers omit `Origin` on same-origin GET requests (for example, a widget preview
on the dashboard host). Only this case may use the configured public widget
origin: the request host must match `PUBLIC_WIDGET_URL`, and browser-owned Fetch
Metadata must say `same-origin` with a `cors` or `same-origin` mode. The resolved
origin still must be explicitly allowed by the installation. Missing Origin
without these checks, navigation requests, cross-site requests, and WebSockets
without Origin are refused. This does not authenticate non-browser clients;
like Origin itself, Fetch Metadata can be forged outside a browser.
