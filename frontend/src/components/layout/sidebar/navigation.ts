export type SidebarSearch = Record<string, string | undefined>;

export type SidebarNavigateTarget =
  | string
  | {
      to: string;
      params?: Record<string, string>;
      search?: Record<string, string>;
    };

export function isSidebarLinkActive(pathname: string, search: SidebarSearch, link: string): boolean {
  const [linkPath, linkQuery] = link.split('?');

  if (linkQuery) {
    if (pathname !== linkPath) {
      return false;
    }

    const params = new URLSearchParams(linkQuery);
    for (const [key, value] of params.entries()) {
      if (search[key] !== value) {
        return false;
      }
    }
    return true;
  }

  if (pathname === linkPath) {
    if (search.collection) {
      return false;
    }
    return true;
  }

  if (link.endsWith('/docs') || link.endsWith('/pm') || link.endsWith('/support')) {
    return false;
  }

  return pathname.startsWith(`${linkPath}/`);
}

export function isTeamSubLinkActive(
  isActive: (link: string) => boolean,
  wsSlug: string,
  activeTeamParam: string | null,
  teamId: string,
  subPath: string,
) {
  return isActive(`/w/${wsSlug}/pm/${subPath}`) && activeTeamParam === teamId;
}
