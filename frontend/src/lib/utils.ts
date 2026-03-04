import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Derive 1-2 character initials from a name or email.
 * "Jane Doe" → "JD", "alice@example.com" → "AL", undefined → "??"
 */
export function getInitials(nameOrEmail?: string | null): string {
  if (!nameOrEmail) return '??';
  const trimmed = nameOrEmail.trim();
  if (!trimmed) return '??';
  const parts = trimmed.split(/\s+/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }
  return trimmed.slice(0, 2).toUpperCase();
}
