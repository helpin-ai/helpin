import { cn } from '@/lib/utils';
import { parseApplyPatch, type PatchFile } from './applyPatchParser';

const MAX_LINES_PER_FILE = 50;

const OP_BADGE: Record<string, { label: string; className: string }> = {
  add: {
    label: 'Added',
    className: 'bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-950/60 dark:text-emerald-400 dark:border-emerald-800/60',
  },
  update: {
    label: 'Modified',
    className: 'bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-950/60 dark:text-blue-400 dark:border-blue-800/60',
  },
  delete: {
    label: 'Deleted',
    className: 'bg-red-50 text-red-700 border border-red-200 dark:bg-red-950/60 dark:text-red-400 dark:border-red-800/60',
  },
};

export function ApplyPatchDiff({ argsText }: { argsText: string }) {
  const parsed = parseApplyPatch(argsText);
  if (!parsed || parsed.files.length === 0) return null;

  return (
    <div className="mt-1.5 flex flex-col gap-2 overflow-hidden rounded-lg border border-border/60">
      {parsed.files.map((file, idx) => (
        <FileDiff key={idx} file={file} />
      ))}
    </div>
  );
}

function FileDiff({ file }: { file: PatchFile }) {
  const badge = OP_BADGE[file.op] ?? OP_BADGE.update;
  const visibleLines = file.lines.slice(0, MAX_LINES_PER_FILE);
  const truncated = file.lines.length - visibleLines.length;

  const displayPath = file.moveTo
    ? `${file.path} → ${file.moveTo}`
    : file.path || 'modified file';

  return (
    <div className="overflow-hidden">
      {/* File header */}
      <div className="flex items-center gap-2.5 border-b border-border/60 bg-muted/60 px-3 py-1.5">
        <span className={cn('rounded px-1.5 py-0.5 text-[10px] font-medium leading-tight', badge.className)}>
          {badge.label}
        </span>
        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-foreground/80">
          {displayPath}
        </span>
      </div>

      {/* Diff lines */}
      {file.op === 'delete' ? (
        <div className="px-3 py-2 font-mono text-[11px] italic text-muted-foreground">
          File deleted
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse font-mono text-[11px] leading-5">
            <tbody>
              {visibleLines.map((line, idx) => (
                <tr
                  key={idx}
                  className={cn(
                    line.type === 'added' && 'bg-green-50 dark:bg-emerald-950/30',
                    line.type === 'removed' && 'bg-red-50 dark:bg-red-950/30',
                  )}
                >
                  <td className={cn(
                    'w-5 select-none px-2 text-center',
                    line.type === 'added' && 'text-green-600 dark:text-emerald-500',
                    line.type === 'removed' && 'text-red-500 dark:text-red-400',
                    line.type === 'context' && 'text-muted-foreground/40',
                  )}>
                    {line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' '}
                  </td>
                  <td className={cn(
                    'whitespace-pre py-0 pr-3',
                    line.type === 'added' && 'text-green-800 dark:text-emerald-300',
                    line.type === 'removed' && 'text-red-700 dark:text-red-300',
                    line.type === 'context' && 'text-foreground/70',
                  )}>
                    {line.text || ' '}
                  </td>
                </tr>
              ))}
              {truncated > 0 ? (
                <tr className="bg-muted/30">
                  <td colSpan={2} className="px-3 py-1 text-[10px] text-muted-foreground">
                    +{truncated} more {truncated === 1 ? 'line' : 'lines'} not shown
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
