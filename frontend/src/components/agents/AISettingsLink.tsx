import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";

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
  return (
    <Link
      className={className}
      to={shared ? "/w/$slug/settings/ai" : "/w/$slug/settings/ai-connections"}
      params={{ slug }}
    >
      {children}
    </Link>
  );
}
