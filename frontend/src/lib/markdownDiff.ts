// LCS-based line diff with word-level inline highlights, used by the docs
// proposal review screen to compare current markdown against proposed markdown.

export interface DiffSegment {
  text: string
  changed: boolean
}

export type DiffRow =
  | { kind: 'kept'; text: string }
  | { kind: 'added' | 'deleted'; text: string; segments: DiffSegment[] }

// Lines beyond this budget (after common prefix/suffix trimming) skip the
// O(n*m) LCS table and fall back to a plain replace of the middle section.
const MAX_LCS_CELLS = 4_000_000

type LineOp = { kind: 'kept' | 'added' | 'deleted'; text: string }

export function buildMarkdownDiff(beforeText: string, afterText: string): DiffRow[] {
  const before = beforeText.split(/\r?\n/).map((line) => line.trimEnd())
  const after = afterText.split(/\r?\n/).map((line) => line.trimEnd())
  const ops = diffLines(before, after)
  return pairChangedLines(ops)
}

function diffLines(before: string[], after: string[]): LineOp[] {
  // Trim the common prefix/suffix so the LCS table only covers the changed core.
  let start = 0
  while (start < before.length && start < after.length && before[start] === after[start]) start += 1
  let endBefore = before.length
  let endAfter = after.length
  while (endBefore > start && endAfter > start && before[endBefore - 1] === after[endAfter - 1]) {
    endBefore -= 1
    endAfter -= 1
  }

  const ops: LineOp[] = []
  for (let i = 0; i < start; i += 1) ops.push({ kind: 'kept', text: before[i]! })

  const midBefore = before.slice(start, endBefore)
  const midAfter = after.slice(start, endAfter)
  if ((midBefore.length + 1) * (midAfter.length + 1) > MAX_LCS_CELLS) {
    for (const text of midBefore) ops.push({ kind: 'deleted', text })
    for (const text of midAfter) ops.push({ kind: 'added', text })
  } else {
    ops.push(...lcsOps(midBefore, midAfter))
  }

  for (let i = endBefore; i < before.length; i += 1) ops.push({ kind: 'kept', text: before[i]! })
  return ops
}

function lcsOps(a: string[], b: string[]): LineOp[] {
  const rows = a.length + 1
  const cols = b.length + 1
  const table = new Uint32Array(rows * cols)
  for (let i = a.length - 1; i >= 0; i -= 1) {
    for (let j = b.length - 1; j >= 0; j -= 1) {
      table[i * cols + j] = a[i] === b[j]
        ? table[(i + 1) * cols + j + 1]! + 1
        : Math.max(table[(i + 1) * cols + j]!, table[i * cols + j + 1]!)
    }
  }
  const ops: LineOp[] = []
  let i = 0
  let j = 0
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      ops.push({ kind: 'kept', text: a[i]! })
      i += 1
      j += 1
    } else if (table[(i + 1) * cols + j]! >= table[i * cols + j + 1]!) {
      ops.push({ kind: 'deleted', text: a[i]! })
      i += 1
    } else {
      ops.push({ kind: 'added', text: b[j]! })
      j += 1
    }
  }
  while (i < a.length) ops.push({ kind: 'deleted', text: a[i++]! })
  while (j < b.length) ops.push({ kind: 'added', text: b[j++]! })
  return ops
}

// Pair each run of deleted lines with the run of added lines that follows it,
// computing word-level highlights for paired lines so small edits read as
// inline changes rather than whole-line replacements.
function pairChangedLines(ops: LineOp[]): DiffRow[] {
  const rows: DiffRow[] = []
  let index = 0
  while (index < ops.length) {
    const op = ops[index]!
    if (op.kind === 'kept') {
      rows.push({ kind: 'kept', text: op.text })
      index += 1
      continue
    }
    const deleted: string[] = []
    const added: string[] = []
    while (index < ops.length && ops[index]!.kind === 'deleted') deleted.push(ops[index++]!.text)
    while (index < ops.length && ops[index]!.kind === 'added') added.push(ops[index++]!.text)
    const paired = Math.min(deleted.length, added.length)
    for (let k = 0; k < deleted.length; k += 1) {
      const segments = k < paired
        ? diffWords(deleted[k]!, added[k]!).before
        : [{ text: deleted[k]!, changed: true }]
      rows.push({ kind: 'deleted', text: deleted[k]!, segments })
    }
    for (let k = 0; k < added.length; k += 1) {
      const segments = k < paired
        ? diffWords(deleted[k]!, added[k]!).after
        : [{ text: added[k]!, changed: true }]
      rows.push({ kind: 'added', text: added[k]!, segments })
    }
  }
  return rows
}

function diffWords(before: string, after: string): { before: DiffSegment[]; after: DiffSegment[] } {
  const beforeTokens = tokenize(before)
  const afterTokens = tokenize(after)
  const ops = lcsOps(beforeTokens, afterTokens)
  const beforeSegments: DiffSegment[] = []
  const afterSegments: DiffSegment[] = []
  for (const op of ops) {
    if (op.kind === 'kept') {
      pushSegment(beforeSegments, op.text, false)
      pushSegment(afterSegments, op.text, false)
    } else if (op.kind === 'deleted') {
      pushSegment(beforeSegments, op.text, true)
    } else {
      pushSegment(afterSegments, op.text, true)
    }
  }
  return { before: beforeSegments, after: afterSegments }
}

function tokenize(line: string): string[] {
  return line.split(/(\s+)/).filter((token) => token !== '')
}

function pushSegment(segments: DiffSegment[], text: string, changed: boolean) {
  const last = segments[segments.length - 1]
  if (last && last.changed === changed) {
    last.text += text
    return
  }
  segments.push({ text, changed })
}
