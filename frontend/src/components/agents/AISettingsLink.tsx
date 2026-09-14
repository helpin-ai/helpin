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
      to="/w/$slug/settings/$section"
      params={{ slug, section: shared ? "ai" : "ai-connections" }}
    >
      {children}
    </Link>
  );
}
