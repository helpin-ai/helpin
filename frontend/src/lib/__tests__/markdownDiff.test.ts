import { describe, expect, it } from 'vitest'
import { buildMarkdownDiff, type DiffRow } from '../markdownDiff'

function kinds(rows: DiffRow[]): string[] {
  return rows.map((row) => row.kind)
}

describe('buildMarkdownDiff', () => {
  it('keeps identical text as kept rows', () => {
    const rows = buildMarkdownDiff('# Title\n\nBody', '# Title\n\nBody')
    expect(kinds(rows)).toEqual(['kept', 'kept', 'kept'])
  })

  it('marks only the inserted line when a line is added in the middle', () => {
    const rows = buildMarkdownDiff('First\nSecond\nThird', 'First\nSecond\nInserted\nThird')
    expect(kinds(rows)).toEqual(['kept', 'kept', 'added', 'kept'])
    expect(rows[2]).toMatchObject({ kind: 'added', text: 'Inserted' })
  })

  it('marks only the removed line when a line is deleted', () => {
    const rows = buildMarkdownDiff('First\nSecond\nThird', 'First\nThird')
    expect(kinds(rows)).toEqual(['kept', 'deleted', 'kept'])
  })

  it('highlights changed words inline on paired changed lines', () => {
    const rows = buildMarkdownDiff('Refunds take 5 days.', 'Refunds take 10 days.')
    expect(kinds(rows)).toEqual(['deleted', 'added'])
    const deleted = rows[0]
    const added = rows[1]
    if (deleted.kind === 'kept' || added.kind === 'kept') throw new Error('expected changed rows')
    expect(deleted.segments.filter((segment) => segment.changed).map((segment) => segment.text)).toEqual(['5'])
    expect(added.segments.filter((segment) => segment.changed).map((segment) => segment.text)).toEqual(['10'])
    expect(deleted.segments.map((segment) => segment.text).join('')).toBe('Refunds take 5 days.')
    expect(added.segments.map((segment) => segment.text).join('')).toBe('Refunds take 10 days.')
  })

  it('does not cascade changes after an insertion', () => {
    const before = ['Intro', 'Alpha', 'Beta', 'Gamma'].join('\n')
    const after = ['Intro', 'New section', 'Alpha', 'Beta', 'Gamma'].join('\n')
    const rows = buildMarkdownDiff(before, after)
    expect(kinds(rows)).toEqual(['kept', 'added', 'kept', 'kept', 'kept'])
  })

  it('treats a full rewrite as delete plus add', () => {
    const rows = buildMarkdownDiff('Old content', 'Entirely different text')
    expect(kinds(rows)).toEqual(['deleted', 'added'])
  })
})
