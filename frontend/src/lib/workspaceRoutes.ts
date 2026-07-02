const WORKSPACE_SUPPORT_ROUTE_RE = /^\/w\/[^/]+\/support(?:\/|$)/;

export function isWorkspaceSupportRoute(pathname: string) {
  return WORKSPACE_SUPPORT_ROUTE_RE.test(pathname);
}
