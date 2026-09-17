/** Preserve explicit HTTP for local deployments and HTTPS for public hosts. */
export function widgetOrigin(host: string): string {
  const url = new URL(/^https?:\/\//.test(host) ? host : `https://${host}`);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '')) {
    throw new Error('Widget host must be an HTTP(S) origin');
  }
  return url.origin;
}

export function widgetURL(host: string, path: string): string {
  return `${widgetOrigin(host)}${path}`;
}

export function widgetSocketURL(host: string, key: string): string {
  return widgetURL(host, `/widget/ws?key=${encodeURIComponent(key)}`).replace(/^http/, 'ws');
}
