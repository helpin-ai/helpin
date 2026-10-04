# Customer portal on the help center

[Operations](README.md)

On the hosted service, each workspace's customer portal is served on its help center at `/requests`:

| Help center address | Portal address |
|---------------------|----------------|
| `https://acme.helpin.center` (hosted subdomain) | `https://acme.helpin.center/requests` |
| `https://help.acme.com` (custom domain) | `https://help.acme.com/requests` |
| `https://acme.com/docs` (customer reverse proxy) | `https://acme.com/docs/requests` |

Customers need no extra DNS, certificates, or settings. The portal reuses the help center's host, TLS, and custom domain. Community installs, and workspaces without a help center, keep the portal in the app at `APP_BASE_URL/portal/{slug}`.

## Configuration

| Where | Variable | Production | Staging |
|-------|----------|------------|---------|
| API server | `HELPCENTER_HOSTED_DOMAIN` | `helpin.center` | `stage.helpin.center` |
| API server | `HELPCENTER_CUSTOM_DOMAIN_TARGET` (optional) | defaults to `HELPCENTER_HOSTED_DOMAIN` | the staging custom-domain entry, if it differs |

When it is unset, portal links and redirects stay in the app. The help center image always includes the portal, but it serves `/requests` only when the API reports a portal for the host, which it does only while this variable is set.

## How it works

- **Build:** `help-center/Dockerfile` builds the portal-only app from `frontend/` (`pnpm --dir frontend run build:portal`, entry `frontend/src/portal/main.tsx`) and copies it into the image at `/app/portal`. The help center image is rebuilt when frontend source changes, because the portal is built from it.
- **Pages:** `help-center/portalMount.mjs` serves `/requests` and its assets.
  - It asks the API which portal the host serves (`GET /api/hc/{subdomain-or-domain}/portal`, cached for 60 seconds).
  - It injects `window.__HELPIN_PORTAL__` and a `<base>` for the help center's base path.
  - Hosts without an enabled portal get 404.
- **API and live updates:**
  - The help center server proxies `/api/public/portal/{slug}/…` to `INTERNAL_API_URL`, but only for the portal that the host serves.
  - The live-update WebSocket (`/api/public/portal/{slug}/ws`) is proxied with the original `Host`, which the API's same-origin check needs.
  - The session cookie stays first-party on the help center host. Under a subpath, its `Path` is rewritten to include the base path.
- **Links:**
  - Sign-in emails, "view your request" links in reply emails, and the address in **Settings → Customer portal** use the help center address plus `/requests`. That address follows the help center's public URL mode: custom domain, reverse proxy, or `{subdomain}.<HELPCENTER_HOSTED_DOMAIN>`.
  - The app's `/portal/{slug}` path redirects there and keeps the page.

## Custom domain verification

A help center custom domain, and so the portal on it, is served only after two DNS records are in place:

| Type | Name | Value | Purpose |
|------|------|-------|---------|
| CNAME | `help.acme.com` | `helpin.center` (the custom domain target) | Routes visitors to Helpin's custom-domain entry (Caddy) |
| TXT | `_helpin-challenge.help.acme.com` | `helpin-verify=<workspace token>` | Proves this workspace owns the domain |

A CNAME alone shows the domain points at Helpin, not which workspace it belongs to. Without the TXT token, a leftover CNAME could be claimed by any workspace, which is a subdomain takeover.

- **Statuses:**
  - `pending`: the domain was entered but isn't proven yet. The help center and portal stay on the hosted subdomain, the TLS ask denies certificates for it, and public config omits it.
  - `verified`: the domain is live.
  - `failing`: the domain was verified but no longer points at Helpin. It keeps serving so a DNS blip doesn't take it offline.
- **When checks run:** saving a new domain checks it immediately, and **Check DNS** in Help center settings re-checks on demand.
- **Background loop:** every 10 minutes the API retries pending domains (each at most hourly) and re-checks live domains daily. Each domain is claimed by one replica per round.
- **Admin alerts:** workspace admins and owners get one notification per change. "Live" and "working again" are in-app only; "stopped pointing to Helpin" also goes by email.
- **Conflicts:** an unverified claim by another workspace gives way when the owner enters the domain. A domain verified by another workspace is refused.
- **Existing domains:** domains in use before this change were marked verified by the migration, and the daily check covers them from then on.
- **Community installs:** with no target configured, custom domains serve as entered, as before.

## Infrastructure

No new services, domains, or certificates are needed.

- **Existing routes:** `*.helpin.center` (nginx, wildcard certificate) and customer custom domains (the Caddy ingress with on-demand TLS, asking `/api/hc/verify-domain`) already route to `helpin-helpcenter-svc`.
- **WebSocket timeouts:** the help center ingresses in `gitops/helpin/{prod,stage}/ingress.yaml` have no read timeout, so nginx closes an idle portal socket after 60 seconds. The portal reconnects and catches up, so updates still arrive, but to keep sockets open add the same annotations the API ingress uses to the help center ingresses:

  ```yaml
  nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
  nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
  ```

  Caddy proxies WebSockets without a timeout.

## Checks after rollout

- `https://{subdomain}.helpin.center/requests` shows the portal sign-in page, and a sign-in link from email opens there.
- The Customer portal settings page shows `https://{subdomain}.helpin.center/requests` as the portal address.
- A help center on a custom domain serves `https://<domain>/requests` over HTTPS.
- Opening `https://app.helpin.ai/portal/{slug}` redirects to the help center address.
