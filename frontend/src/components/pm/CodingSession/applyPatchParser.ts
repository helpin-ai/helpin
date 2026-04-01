export type PatchLineType = 'added' | 'removed' | 'context';

export interface PatchLine {
  type: PatchLineType;
  text: string;
}

export type PatchFileOp = 'add' | 'update' | 'delete';

export interface PatchFile {
  op: PatchFileOp;
  path: string;
  moveTo?: string;
  lines: PatchLine[];
}

export interface ParsedPatch {
  files: PatchFile[];
}

// ─── *** Begin Patch format (native tool executor) ───────────────────────────

function parseBeginPatchFormat(argsText: string): ParsedPatch | null {
  if (!argsText.includes('*** Begin Patch')) return null;

  const lines = argsText.split('\n');
  const files: PatchFile[] = [];
  let current: PatchFile | null = null;

  for (const raw of lines) {
    const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw;

    if (line === '*** Begin Patch' || line === '*** End Patch') continue;

    if (line.startsWith('*** Add File: ')) {
      current = { op: 'add', path: line.slice('*** Add File: '.length).trim(), lines: [] };
      files.push(current);
      continue;
    }
    if (line.startsWith('*** Update File: ')) {
      current = { op: 'update', path: line.slice('*** Update File: '.length).trim(), lines: [] };
      files.push(current);
      continue;
    }
    if (line.startsWith('*** Delete File: ')) {
      current = { op: 'delete', path: line.slice('*** Delete File: '.length).trim(), lines: [] };
      files.push(current);
      continue;
    }
    if (line.startsWith('*** Move to: ') && current) {
      current.moveTo = line.slice('*** Move to: '.length).trim();
      continue;
    }
    if (line.startsWith('@@')) continue;
    if (!current || current.op === 'delete') continue;

    if (line.startsWith('+')) {
      current.lines.push({ type: 'added', text: line.slice(1) });
    } else if (line.startsWith('-')) {
      current.lines.push({ type: 'removed', text: line.slice(1) });
    } else {
      current.lines.push({ type: 'context', text: line.startsWith(' ') ? line.slice(1) : line });
    }
  }

  return { files };
}

// ─── Standard unified diff format (codex, git) ───────────────────────────────

function stripDiffPrefix(path: string): string {
  // Strip a/ or b/ prefix added by git
  if (path.startsWith('a/') || path.startsWith('b/')) return path.slice(2);
  return path;
}

function parseUnifiedDiffFormat(argsText: string): ParsedPatch | null {
  const lines = argsText.split('\n');
  const files: PatchFile[] = [];
  let current: PatchFile | null = null;
  let inHunk = false;

  for (const raw of lines) {
    const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw;

    // New file header: "diff --git a/... b/..." or "diff -u ..."
    if (line.startsWith('diff ')) {
      inHunk = false;
      current = null;
      continue;
    }

    // Old file path: "--- a/path" or "--- path" or "--- /dev/null" (new file)
    if (line.startsWith('--- ')) {
      inHunk = false;
      const raw_path = line.slice(4).trim();
      if (raw_path === '/dev/null') {
        // Will be set properly by +++ line
        current = { op: 'add', path: '', lines: [] };
      } else {
        current = { op: 'update', path: stripDiffPrefix(raw_path), lines: [] };
      }
      files.push(current);
      continue;
    }

    // New file path: "+++ b/path" or "+++ path" or "+++ /dev/null" (deleted file)
    if (line.startsWith('+++ ')) {
      const raw_path = line.slice(4).trim();
      if (raw_path === '/dev/null') {
        if (current) current.op = 'delete';
      } else if (current) {
        const path = stripDiffPrefix(raw_path);
        if (current.op === 'add') {
          current.path = path;
        } else if (current.op === 'update' && current.path !== path) {
          // rename
          current.moveTo = path;
        }
      }
      continue;
    }

    // Hunk header: "@@ -X,Y +X,Z @@ optional context"
    if (line.startsWith('@@')) {
      inHunk = true;
      // If we never saw --- / +++ headers (bare hunk), create a placeholder file
      if (!current) {
        current = { op: 'update', path: '', lines: [] };
        files.push(current);
      }
      continue;
    }

    if (!inHunk || !current || current.op === 'delete') continue;

    if (line.startsWith('+')) {
      current.lines.push({ type: 'added', text: line.slice(1) });
    } else if (line.startsWith('-')) {
      current.lines.push({ type: 'removed', text: line.slice(1) });
    } else if (line.startsWith(' ')) {
      current.lines.push({ type: 'context', text: line.slice(1) });
    } else if (line === '\\ No newline at end of file') {
      // skip
    } else if (line.trim() !== '') {
      // unexpected line outside a hunk — stop hunk mode
      inHunk = false;
    }
  }

  return files.length > 0 ? { files } : null;
}

// ─── Public API ──────────────────────────────────────────────────────────────

export function parseApplyPatch(argsText: string): ParsedPatch | null {
  const trimmed = argsText.trim();
  if (!trimmed) return null;

  return parseBeginPatchFormat(trimmed) ?? parseUnifiedDiffFormat(trimmed);
}
