export function parseEmailList(value: string): string[] {
  return [...new Set(
    value
      .split(/[;,\s]+/)
      .map((email) => email.trim().toLowerCase())
      .filter(Boolean),
  )]
}
