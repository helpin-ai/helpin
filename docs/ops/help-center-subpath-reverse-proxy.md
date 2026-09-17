# Help-center subpath proxy design and setup

> Status: this page combines an April design plan with integration examples.
> Subpath context is now implemented in [the SSR server](../../help-center/serve.mjs)
> and covered by [context tests](../../help-center/src/lib/__tests__/utils.test.ts).
> [Help-center settings](../../frontend/src/components/settings/HelpcenterTab.tsx)
> and [model fields](../../server/internal/model/docs.go) include reverse-proxy
> configuration. Treat the phases below as design history, not unfinished tasks;
> validate the customer proxy and selected deployment before cutover.


**Date:** 2026-04-23
**Scope:** Public Helpin help centers mounted under a customer's existing website path, for example `https://usermaven.com/docs`, `https://usermaven.com/help`, or `https://usermaven.com/help-center`.
**Goal:** Let customers reverse proxy their Helpin help center from a subpath while preserving SEO, search, feedback, assets, and internal navigation.

---

## 1. Recommended Architecture

Use the tenant's raw Helpin-hosted origin as the upstream:

```txt
https://<help-center-subdomain>.helpin.center
```

Example:

```txt
Customer URL: https://usermaven.com/docs
Proxy origin: https://usermaven.helpin.center
```

The public base path is customer-selected. Use `/docs` in examples below as the default, but the same contract must work for `/help-center`, `/help`, `/resources`, or any other normalized single-path mount the customer chooses.

Do not proxy through the old custom docs hostname, for example `https://docs.usermaven.com`, if that hostname will later redirect to `https://usermaven.com/docs`. That creates a redirect/proxy loop. The raw Helpin host avoids the loop and removes one DNS/TLS hop.

The proxy must provide three pieces of request context to Helpin:

| Context | Example | Why it matters |
|---|---|---|
| Tenant | `usermaven` | Selects the correct Helpin help center. |
| Public host | `usermaven.com` | Builds canonical URLs, Open Graph URLs, sitemap links, alternate locale links, and redirects. |
| Public base path | `/docs`, `/help`, `/help-center` | Prefixes internal links, assets, API calls, sitemap, robots, and canonical paths. |

The current server resolves explicit tenant/base-path context from the supported
headers or query parameters, validates tenant selection against the raw host,
and carries the resulting base path through routing and asset URLs. The
implementation plan below records the original design.

---

## 2. Product Requirements

### Required Behavior

- `https://customer.com/<base-path>` renders the help center homepage.
- `https://customer.com/<base-path>/c/<collection>` renders a collection.
- `https://customer.com/<base-path>/articles/<article-key>` renders an article.
- All Helpin-generated links stay under the configured base path.
- Static assets, API calls, search, article feedback, sitemap, robots, and locale links work through the customer domain.
- Canonical URLs and metadata use `https://customer.com/<base-path>/...`, not `https://<tenant>.helpin.center/...`.
- The raw Helpin origin remains available for proxying, but should not be advertised as the canonical public URL when reverse-proxy mode is configured.
- Redirects are one-way only. If `docs.customer.com` is retired, it redirects to `customer.com/docs`, and the proxy origin stays `tenant.helpin.center`.

### Non-Goals

- Do not ask customers to proxy all root-level routes such as `/c/*` or `/articles/*` unless unavoidable. That collides with the customer's website.
- Do not require customers to expose Helpin under their apex root.
- Do not rely on `docs.customer.com` as the upstream target after it starts redirecting.

---

## 3. Helpin Implementation Plan

### Phase 1: Add Explicit Proxy Context

Add a first-class reverse-proxy context that can be supplied by trusted proxy headers or explicit origin query parameters:

```txt
X-Helpin-HC-Tenant: usermaven
X-Helpin-HC-Basepath: <base-path>
X-Forwarded-Host: usermaven.com
X-Forwarded-Proto: https
```

Rules:

- Tenant lookup uses `X-Helpin-HC-Tenant` when present, then falls back to the current host-based resolver.
- Public origin uses `X-Forwarded-Host` and `X-Forwarded-Proto`.
- Public base path uses `X-Helpin-HC-Basepath`.
- Normalize base paths: no trailing slash, must start with `/`, reject `//`, `..`, encoded slash tricks, and empty segments.
- Allow query fallback for platforms that cannot set upstream headers:

```txt
https://usermaven.helpin.center/?helpin_tenant=usermaven&helpin_basepath=<base-path>
```

If query fallback is added, keep it non-secret and validate it against the upstream hostname so one customer cannot select another customer's help center through an arbitrary public URL.

### Phase 2: Make SSR And Client Routing Basepath-Aware

Update:

- `help-center/serve.mjs`
- `help-center/src/lib/utils.ts`
- `help-center/src/lib/requestContext.ts`
- `help-center/src/router.tsx`
- SEO helpers in `help-center/src/lib/seo.ts` and `help-center/src/lib/alternateLinks.ts`
- `robots.txt`, `sitemap.xml`, `llms.txt`, and any generated absolute links

Expected output for `basepath = /help-center`:

```txt
Homepage:       /help-center
Collection:     /help-center/c/getting-started
Article:        /help-center/articles/installing-helpin-abc123ef
Search:         /help-center/search
Sitemap:        /help-center/sitemap.xml
Assets:         /help-center/assets/...
Client API:     /help-center/api/hc/...
Canonical URL:  https://usermaven.com/help-center/...
```

The current helper `prefixBasepath()` should be the central path builder. Avoid one-off string concatenation in route components.

### Phase 3: Stabilize Proxy Prefixes

Document the only prefixes a customer must proxy:

```txt
/<base-path>
/<base-path>/*
```

When Helpin is basepath-aware, the customer should not need root-level rewrites for `/assets`, `/api`, `/c`, `/articles`, `/search`, `/sitemap.xml`, or `/robots.txt`. Those should all be emitted under the configured base path.

Until then, a customer may need temporary compatibility rewrites for root-relative assets/API calls, but that should be treated as a workaround, not the long-term contract.

### Phase 4: Add Tests

Add focused tests for:

- Host `usermaven.helpin.center`, forwarded host `usermaven.com`, configured base path resolves tenant `usermaven`.
- Canonical SEO URLs use `https://usermaven.com/<base-path>/...`.
- Internal links are prefixed with the configured base path.
- Assets in rendered HTML are rewritten to `/<base-path>/assets/...`.
- API calls use `/<base-path>/api/...` in the browser and are stripped back to `/api/...` by the server/proxy.
- Sitemap and robots include the configured base path.
- Multilingual paths work, for example `/<base-path>/fr/c/...`.
- Retired custom domain redirects do not loop through the proxy origin.

### Phase 5: Admin Surface

Add settings to the help-center custom domain panel:

- Public URL mode: `Hosted subdomain`, `Custom domain`, `Reverse proxy subpath`.
- Reverse proxy public URL: `https://customer.com/<base-path>`.
- Raw proxy origin: read-only generated value, for example `https://usermaven.helpin.center`.
- Verification status:
  - origin reachable
  - `/<base-path>` returns 200
  - `/<base-path>/assets/...` returns 200
  - canonical URL uses customer domain
  - sitemap reachable
  - no redirect loop detected

---

## 4. Customer Guide: Next.js On Vercel

This applies when the customer's marketing website is a Next.js app deployed on Vercel and they want a subpath such as `https://customer.com/docs`, `https://customer.com/help`, or `https://customer.com/help-center`.

### Prerequisites

- Helpin help center is published.
- Raw Helpin origin works directly:

```txt
https://usermaven.helpin.center
```

- Helpin reverse-proxy mode is configured with:

```txt
Tenant: usermaven
Public URL: https://usermaven.com/docs
Base path: /docs
```

### Rewrites

In `next.config.mjs`:

```js
const DOCS_ORIGIN =
  process.env.DOCS_WEBSITE_URL || 'https://usermaven.helpin.center'
const DOCS_TENANT = process.env.DOCS_HELPIN_TENANT || 'usermaven'
const DOCS_BASE_PATH = process.env.DOCS_HELPIN_BASE_PATH || '/docs'

export default {
  async rewrites() {
    const proxyContext =
      `helpin_tenant=${DOCS_TENANT}&helpin_basepath=${encodeURIComponent(DOCS_BASE_PATH)}`

    return [
      {
        source: DOCS_BASE_PATH,
        destination: `${DOCS_ORIGIN}/?${proxyContext}`,
      },
      {
        source: `${DOCS_BASE_PATH}/:path*`,
        destination: `${DOCS_ORIGIN}/:path*?${proxyContext}`,
      },
    ]
  },
}
```

Set the Vercel environment variable in Production and Preview:

```txt
DOCS_WEBSITE_URL=https://usermaven.helpin.center
DOCS_HELPIN_TENANT=usermaven
DOCS_HELPIN_BASE_PATH=/docs
```

For a different mount such as `https://usermaven.com/help-center`, set `DOCS_HELPIN_BASE_PATH=/help-center`. The generated browser requests then become `/help-center/assets/...`, `/help-center/api/...`, and `/help-center/articles/...`.

If the site uses `output: 'export'`, Next.js rewrites do not run as a server feature. Use Vercel project rewrites, a Vercel edge function, Cloudflare Worker, nginx, or another runtime proxy instead.

The Vercel rewrite variant uses query parameters because `next.config.mjs` rewrites are the simplest path-only proxy surface. Helpin validates the tenant against the raw `*.helpin.center` upstream host, and the browser never sees the rewritten origin URL.

### Header-Aware Proxy Variant

If the customer uses Next.js Middleware or another edge proxy that can set upstream request headers, send explicit Helpin context:

```txt
X-Helpin-HC-Tenant: usermaven
X-Helpin-HC-Basepath: <base-path>
X-Forwarded-Host: usermaven.com
X-Forwarded-Proto: https
```

This is preferred because it separates tenant lookup from public URL generation.

### What Not To Add

Do not add this no-op rewrite:

```js
{ source: '/:path*', destination: '/:path*' }
```

It does not change routing behavior and makes the config harder to audit.

Do not point `DOCS_WEBSITE_URL` at `https://docs.usermaven.com` if that domain redirects to `https://usermaven.com/docs`.

---

## 5. Customer Guide: nginx

```nginx
location = /docs {
  proxy_pass https://usermaven.helpin.center/;
  proxy_ssl_server_name on;
  proxy_set_header Host usermaven.helpin.center;
  proxy_set_header X-Helpin-HC-Tenant usermaven;
  proxy_set_header X-Helpin-HC-Basepath /docs;
  proxy_set_header X-Forwarded-Host $host;
  proxy_set_header X-Forwarded-Proto $scheme;
}

location /docs/ {
  rewrite ^/docs/(.*)$ /$1 break;
  proxy_pass https://usermaven.helpin.center;
  proxy_ssl_server_name on;
  proxy_set_header Host usermaven.helpin.center;
  proxy_set_header X-Helpin-HC-Tenant usermaven;
  proxy_set_header X-Helpin-HC-Basepath /docs;
  proxy_set_header X-Forwarded-Host $host;
  proxy_set_header X-Forwarded-Proto $scheme;
}
```

---

## 6. Customer Guide: Cloudflare Worker

```js
export default {
  async fetch(request) {
    const url = new URL(request.url)
    const HELPIN_BASE_PATH = '/docs'

    if (url.pathname === HELPIN_BASE_PATH) {
      url.pathname = '/'
    } else if (url.pathname.startsWith(`${HELPIN_BASE_PATH}/`)) {
      url.pathname = url.pathname.slice(HELPIN_BASE_PATH.length)
    } else {
      return fetch(request)
    }

    url.hostname = 'usermaven.helpin.center'

    const headers = new Headers(request.headers)
    headers.set('X-Helpin-HC-Tenant', 'usermaven')
    headers.set('X-Helpin-HC-Basepath', HELPIN_BASE_PATH)
    headers.set('X-Forwarded-Host', 'usermaven.com')
    headers.set('X-Forwarded-Proto', 'https')

    const init = {
      method: request.method,
      headers,
      redirect: 'manual',
    }

    if (request.method !== 'GET' && request.method !== 'HEAD') {
      init.body = request.body
    }

    return fetch(url.toString(), init)
  },
}
```

The Worker does not need to set the `Host` header manually; the upstream URL hostname controls the origin host.

---

## 7. Asset And API Prefix Discovery

For each customer setup, verify what the origin emits before cutover.

In browser DevTools:

1. Open the proxied preview URL, for example `https://preview.customer.com/docs` or `https://preview.customer.com/help-center`.
2. Open Network tab.
3. Filter by `All`.
4. Hard reload.
5. Record any request that leaves the configured base path.

Expected first-class reverse-proxy behavior:

```txt
/<base-path>/assets/...
/<base-path>/api/...
/<base-path>/c/...
/<base-path>/articles/...
/<base-path>/search
/<base-path>/sitemap.xml
/<base-path>/robots.txt
```

In this mode, separate root-level asset rewrites are not part of the contract. If the base path is `/help-center`, the browser requests `/help-center/assets/...`, the customer proxy sends that to `https://usermaven.helpin.center/assets/...`, and Helpin serves those files with long-lived immutable cache headers. Search, feedback, and config calls follow the same shape through `/<base-path>/api/...`.

If you see root-level paths, treat them as bugs to fix in Helpin:

```txt
/assets/...
/api/...
/c/...
/articles/...
/search
/sitemap.xml
/robots.txt
```

Temporary compatibility rewrites can cover root-level `/assets` and `/api`, but they are risky for `/c`, `/articles`, and `/search` because those may collide with the customer's own site routes.

---

## 8. Cutover Sequence

1. Confirm raw origin works:

```bash
curl -I https://usermaven.helpin.center
```

2. Enable Helpin reverse-proxy mode:

```txt
Tenant: usermaven
Public URL: https://usermaven.com/<base-path>
Base path: <base-path>
```

3. Deploy the customer proxy on preview.
4. Verify HTML, assets, API calls, search, feedback, sitemap, robots, canonical tags, Open Graph tags, and internal navigation.
5. Verify old docs domain redirect:

```txt
https://docs.usermaven.com/* -> https://usermaven.com/docs/*
```

6. Confirm the proxy origin is still `https://usermaven.helpin.center`, not `https://docs.usermaven.com`.
7. Ship production proxy.
8. Turn on the old-domain redirect.
9. Monitor 404s, 5xxs, and redirect chains for 24-48 hours.

---

## 9. Verification Checklist

- [ ] `GET /<base-path>` returns 200.
- [ ] `GET /<base-path>/` returns 200 or redirects once to `/<base-path>`.
- [ ] Collection links stay under `/<base-path>/c/...`.
- [ ] Article links stay under `/<base-path>/articles/...`.
- [ ] Search page stays under `/<base-path>/search`.
- [ ] Search API calls succeed.
- [ ] Article feedback POST succeeds.
- [ ] CSS and JS load from `/<base-path>/assets/...`.
- [ ] No browser request goes to `tenant.helpin.center` directly.
- [ ] Canonical tags use `https://customer.com/<base-path>/...`.
- [ ] Open Graph URLs use `https://customer.com/<base-path>/...`.
- [ ] Sitemap URLs use `https://customer.com/<base-path>/...`.
- [ ] Retired docs domain redirects to `customer.com/<base-path>` without a loop.
- [ ] `tenant.helpin.center` does not redirect to `docs.customer.com`.
- [ ] Multilingual locale switcher preserves the configured base path.
- [ ] Browser back/forward navigation works.
- [ ] Hard refresh on a nested article works.
- [ ] 404 pages render inside the customer's configured base path.

---

## 10. Common Failure Modes

| Symptom | Likely cause | Fix |
|---|---|---|
| Page loads, but CSS/JS 404s | Assets emitted at `/assets` while only the configured base path is proxied | Make Helpin basepath-aware; temporary proxy `/assets/*` only if it does not collide. |
| Clicking an article leaves the configured base path | Internal links emitted without base path | Use `prefixBasepath()` everywhere and pass the configured base path into request context. |
| Origin returns "help center not found" | Proxy sent `X-Forwarded-Host: customer.com` and Helpin used that for tenant lookup | Send explicit tenant context or set upstream `Host` to `tenant.helpin.center` and ensure Helpin separates tenant lookup from public host. |
| Canonical tag points at `tenant.helpin.center` | Origin only sees raw Helpin host | Send public forwarded host/proto and basepath. |
| Redirect loop after cutover | Proxy origin points at retired `docs.customer.com` | Use raw Helpin origin as upstream. |
| Website API conflicts with help-center API | Both use `/api/*` at the customer root | In first-class mode, browser API calls should be `/<base-path>/api/*`; avoid root-level `/api` rewrites. |
| Vercel rewrites do nothing | Site is statically exported | Use runtime rewrites, Vercel routing, middleware, Cloudflare Worker, or nginx. |

---

## 11. Internal Acceptance Criteria

Helpin can mark subpath reverse proxy support as ready when:

- A customer only has to proxy the configured base path and its children, for example `/docs` and `/docs/*` or `/help-center` and `/help-center/*`.
- The customer does not need to discover and proxy root-level asset/API/page prefixes.
- Tenant lookup, public host, and public base path are independent.
- All SEO surfaces use the customer URL.
- Redirect loop prevention is documented and tested.
- The admin UI provides a copyable raw origin and a verification checklist.
