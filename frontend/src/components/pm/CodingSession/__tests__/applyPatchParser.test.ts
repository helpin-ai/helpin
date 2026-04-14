import { describe, expect, it } from 'vitest';

import { parseApplyPatch } from '../applyPatchParser';

describe('parseApplyPatch', () => {
  it('returns null for empty input', () => {
    expect(parseApplyPatch('')).toBeNull();
    expect(parseApplyPatch('   ')).toBeNull();
  });

  describe('*** Begin Patch format', () => {
    it('parses an Update File block with added/removed/context lines', () => {
      const patch = [
        '*** Begin Patch',
        '*** Update File: src/example.ts',
        '@@',
        ' const x = 1;',
        '-const y = 2;',
        '+const y = 3;',
        ' const z = 4;',
        '*** End Patch',
      ].join('\n');

      const result = parseApplyPatch(patch);
      expect(result).not.toBeNull();
      expect(result!.files).toHaveLength(1);

      const [file] = result!.files;
      expect(file.op).toBe('update');
      expect(file.path).toBe('src/example.ts');
      expect(file.lines).toEqual([
        { type: 'context', text: 'const x = 1;' },
        { type: 'removed', text: 'const y = 2;' },
        { type: 'added', text: 'const y = 3;' },
        { type: 'context', text: 'const z = 4;' },
      ]);
    });

    it('parses Add File and Delete File operations', () => {
      const patch = [
        '*** Begin Patch',
        '*** Add File: src/new.ts',
        '+export const added = true;',
        '*** Delete File: src/old.ts',
        '*** End Patch',
      ].join('\n');

      const result = parseApplyPatch(patch);
      expect(result!.files).toHaveLength(2);
      expect(result!.files[0].op).toBe('add');
      expect(result!.files[0].path).toBe('src/new.ts');
      expect(result!.files[0].lines).toEqual([{ type: 'added', text: 'export const added = true;' }]);
      expect(result!.files[1].op).toBe('delete');
      expect(result!.files[1].path).toBe('src/old.ts');
    });

    it('captures Move to target for renames', () => {
      const patch = [
        '*** Begin Patch',
        '*** Update File: src/old-name.ts',
        '*** Move to: src/new-name.ts',
        '@@',
        ' unchanged',
        '*** End Patch',
      ].join('\n');

      const [file] = parseApplyPatch(patch)!.files;
      expect(file.path).toBe('src/old-name.ts');
      expect(file.moveTo).toBe('src/new-name.ts');
    });
  });

  describe('unified diff format (Codex / git)', () => {
    it('strips a/ and b/ prefixes from paths', () => {
      const patch = [
        'diff --git a/src/foo.ts b/src/foo.ts',
        '--- a/src/foo.ts',
        '+++ b/src/foo.ts',
        '@@ -1,3 +1,3 @@',
        ' const a = 1;',
        '-const b = 2;',
        '+const b = 3;',
        ' const c = 4;',
      ].join('\n');

      const [file] = parseApplyPatch(patch)!.files;
      expect(file.op).toBe('update');
      expect(file.path).toBe('src/foo.ts');
      expect(file.lines).toHaveLength(4);
      expect(file.lines.filter((l) => l.type === 'added')).toHaveLength(1);
      expect(file.lines.filter((l) => l.type === 'removed')).toHaveLength(1);
    });

    it('detects new files via /dev/null and deleted files via +++ /dev/null', () => {
      const addPatch = [
        '--- /dev/null',
        '+++ b/src/created.ts',
        '@@ -0,0 +1,1 @@',
        '+new file contents',
      ].join('\n');
      const deletePatch = [
        '--- a/src/removed.ts',
        '+++ /dev/null',
        '@@ -1,1 +0,0 @@',
        '-gone',
      ].join('\n');

      const added = parseApplyPatch(addPatch)!.files[0];
      expect(added.op).toBe('add');
      expect(added.path).toBe('src/created.ts');

      const deleted = parseApplyPatch(deletePatch)!.files[0];
      expect(deleted.op).toBe('delete');
    });

    it('treats differing --- and +++ paths as a rename', () => {
      const patch = [
        '--- a/old.ts',
        '+++ b/new.ts',
        '@@ -1,1 +1,1 @@',
        ' same line',
      ].join('\n');

      const [file] = parseApplyPatch(patch)!.files;
      expect(file.op).toBe('update');
      expect(file.path).toBe('old.ts');
      expect(file.moveTo).toBe('new.ts');
    });

    it('handles multiple hunks in a single file', () => {
      const patch = [
        '--- a/src/multi.ts',
        '+++ b/src/multi.ts',
        '@@ -1,2 +1,2 @@',
        ' first',
        '+inserted-early',
        '@@ -20,1 +21,1 @@',
        '-removed-late',
        '+added-late',
      ].join('\n');

      const [file] = parseApplyPatch(patch)!.files;
      expect(file.lines).toEqual([
        { type: 'context', text: 'first' },
        { type: 'added', text: 'inserted-early' },
        { type: 'removed', text: 'removed-late' },
        { type: 'added', text: 'added-late' },
      ]);
    });

    it('ignores "\\ No newline at end of file" markers', () => {
      const patch = [
        '--- a/a.txt',
        '+++ b/a.txt',
        '@@ -1 +1 @@',
        '-one',
        '\\ No newline at end of file',
        '+two',
        '\\ No newline at end of file',
      ].join('\n');

      const [file] = parseApplyPatch(patch)!.files;
      expect(file.lines).toEqual([
        { type: 'removed', text: 'one' },
        { type: 'added', text: 'two' },
      ]);
    });

    it('parses the real Codex apply_patch payload from the pm_checklist_item.go bug report', () => {
      // Minimal slice of the production payload from the bug report.
      // The full diff had multiple hunks; this covers the context/added/removed mix.
      const realPayload = [
        '--- a/server/internal/service/pm_checklist_item.go',
        '+++ b/server/internal/service/pm_checklist_item.go',
        '@@ -10,2 +10,3 @@',
        ' \t"github.com/helpin-ai/helpin/server/internal/repository"',
        '+\t"github.com/helpin-ai/helpin/server/internal/tiptap"',
        ' \t"github.com/helpin-ai/helpin/server/internal/websocket"',
        '@@ -52,3 +53,3 @@',
        ' \t}',
        '-\tif strings.TrimSpace(req.Text) == "" {',
        '+\tif !checklistContentHasVisibleContent(req.Text) {',
        ' \t\treturn nil, fmt.Errorf("text is required")',
      ].join('\n');

      const result = parseApplyPatch(realPayload);
      expect(result).not.toBeNull();
      expect(result!.files).toHaveLength(1);

      const [file] = result!.files;
      expect(file.op).toBe('update');
      expect(file.path).toBe('server/internal/service/pm_checklist_item.go');
      expect(file.lines.some((l) => l.type === 'added' && l.text.includes('tiptap'))).toBe(true);
      expect(file.lines.some((l) => l.type === 'removed' && l.text.includes('TrimSpace'))).toBe(true);
      expect(file.lines.some((l) => l.type === 'added' && l.text.includes('checklistContentHasVisibleContent'))).toBe(true);
    });
  });

  describe('format detection', () => {
    it('prefers *** Begin Patch format when both markers could match', () => {
      const patch = [
        '*** Begin Patch',
        '*** Update File: a.ts',
        ' context-line',
        '*** End Patch',
      ].join('\n');
      const result = parseApplyPatch(patch);
      expect(result!.files[0].path).toBe('a.ts');
    });

    it('returns null when no recognisable diff markers are present', () => {
      expect(parseApplyPatch('just a plain text blob\nwith no diff')).toBeNull();
    });
  });
});
