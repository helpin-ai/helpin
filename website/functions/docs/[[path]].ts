// Cloudflare Pages Function: serves the Helpin help center at helpin.ai/docs.
//
// Reverse proxies /docs and /docs/* to the raw hosted origin (never the
// docs.helpin.ai custom domain, to avoid redirect loops) and passes the
// subpath context the help-center server expects. See
// docs/ops/help-center-subpath-reverse-proxy.md.
//
// Optional Pages env overrides: DOCS_ORIGIN, DOCS_TENANT.

const BASE_PATH = '/docs';
const DEFAULT_ORIGIN = 'https://docs.helpin.center';
const DEFAULT_TENANT = 'docs';

interface Env {
  DOCS_ORIGIN?: string;
  DOCS_TENANT?: string;
}

interface PagesContext {
  request: Request;
  env: Env;
}

export async function onRequest({ request, env }: PagesContext): Promise<Response> {
  const url = new URL(request.url);
  const origin = new URL(env.DOCS_ORIGIN || DEFAULT_ORIGIN);

  const upstream = new URL(url.pathname.slice(BASE_PATH.length) || '/', origin);
  upstream.search = url.search;

  const headers = new Headers(request.headers);
  headers.delete('host');
  headers.set('X-Helpin-HC-Tenant', env.DOCS_TENANT || DEFAULT_TENANT);
  headers.set('X-Helpin-HC-Basepath', BASE_PATH);
  headers.set('X-Forwarded-Host', url.host);
  headers.set('X-Forwarded-Proto', url.protocol.replace(/:$/, ''));

  const hasBody = request.method !== 'GET' && request.method !== 'HEAD';
  return fetch(upstream.toString(), {
    method: request.method,
    headers,
    body: hasBody ? request.body : undefined,
    redirect: 'manual',
  });
}
