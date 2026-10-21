export interface DockReferenceMention {
  start: number;
  end: number;
  query: string;
}

/** A standalone @ token at the caret; embedded @ characters remain ordinary text. */
export function dockReferenceMention(value: string, start: number, end: number): DockReferenceMention | null {
  if (start !== end) return null;
  const match = /(?:^|\s)@([^\s@]*)$/.exec(value.slice(0, start));
  if (!match) return null;
  const tokenStart = start - match[1].length - 1;
  const suffix = /^[^\s]*/.exec(value.slice(start))![0];
  if (suffix.includes('@')) return null;
  return {
    start: tokenStart,
    end: start + suffix.length,
    query: value.slice(tokenStart + 1, start + suffix.length),
  };
}
