/** Match the server's (created_at, id) order, including PostgreSQL microseconds. */
export function compareMessageOrder(aTime: string, aID: string, bTime: string, bID: string): number {
  const aMillis = Date.parse(aTime);
  const bMillis = Date.parse(bTime);
  if (!Number.isFinite(aMillis) || !Number.isFinite(bMillis)) return 0;
  if (aMillis !== bMillis) return aMillis - bMillis;
  // Date.parse drops submillisecond digits. Pad fractions so .12 and .120000
  // compare equally, including timestamps expressed with a timezone offset.
  const fraction = (value: string) => Number((value.match(/\.(\d+)(?:Z|[+-]\d{2}:\d{2})$/)?.[1] ?? '').padEnd(9, '0').slice(3, 9));
  return fraction(aTime) - fraction(bTime) || (aID < bID ? -1 : aID > bID ? 1 : 0);
}
