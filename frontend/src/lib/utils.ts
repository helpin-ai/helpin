import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { format, parseISO, differenceInMinutes, differenceInHours, differenceInDays, isThisYear } from "date-fns"

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

/**
 * Smart relative/absolute time formatting.
 * - < 1 min: "Just now"
 * - < 60 min: "Xm ago"
 * - < 24 hours: "Xh ago"
 * - < 7 days: "Xd ago"
 * - Same year: "Mar 8, 2:30 PM"
 * - Older: "Mar 8, 2025"
 */
export function timeAgo(isoOrDate: string | Date): string {
  const date = typeof isoOrDate === 'string' ? parseISO(isoOrDate) : isoOrDate;
  const now = new Date();
  const mins = differenceInMinutes(now, date);
  if (mins < 1) return 'Just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = differenceInHours(now, date);
  if (hours < 24) return `${hours}h ago`;
  const days = differenceInDays(now, date);
  if (days < 7) return `${days}d ago`;
  if (isThisYear(date)) return format(date, 'MMM d, h:mm a');
  return format(date, 'MMM d, yyyy');
}
