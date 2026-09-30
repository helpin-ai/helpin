/**
 * Derive 1-2 character initials from a name or email.
 * "Jane Mary Doe" → "JD", "Jane" → "J", "alice@example.com" → "A".
 */
export function getInitials(nameOrEmail?: string | null): string {
  if (!nameOrEmail) return '?';
  const trimmed = nameOrEmail.trim();
  if (!trimmed) return '?';
  const parts = trimmed.split(/\s+/);
  if (parts.length >= 2) {
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }
  return Array.from(trimmed)[0].toUpperCase();
}
