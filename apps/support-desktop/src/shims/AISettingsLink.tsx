import type { ReactNode } from "react";
import { buildWorkspaceWebUrl, openWorkspaceWebUrl } from "@desktop/lib/webApp";

export function AISettingsLink({
  slug,
  shared = false,
  children,
  className,
}: {
  slug: string;
  shared?: boolean;
  children: ReactNode;
  className?: string;
}) {
  const path = `/w/${encodeURIComponent(slug)}/settings/${shared ? "ai" : "ai-connections"}`;
  return (
    <a
      className={className}
      href={buildWorkspaceWebUrl(path)}
      onClick={(event) => {
        event.preventDefault();
        openWorkspaceWebUrl(path);
      }}
    >
      {children}
    </a>
  );
}
