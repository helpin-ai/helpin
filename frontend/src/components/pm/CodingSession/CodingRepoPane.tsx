import { SourceCodeIcon, GitBranchIcon, Loading01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { CodingSessionDiff, CodingSessionRepoFile, CodingSessionRepoState } from '@/lib/pmTypes';

export function CodingRepoPane({
  repo,
  files,
  selectedFile,
  onSelectFile,
  diff,
  diffLoading = false,
}: {
  repo: CodingSessionRepoState | null;
  files: CodingSessionRepoFile[];
  selectedFile: string | null;
  onSelectFile: (path: string) => void;
  diff: CodingSessionDiff | null;
  diffLoading?: boolean;
}) {
  return (
    <section className="flex h-full min-h-[15rem] flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm xl:min-h-0">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <SourceCodeIcon className="h-3.5 w-3.5" />
          Repository cockpit
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="text-[10px]">
            {files.length} files
          </Badge>
          {repo?.branch ? (
            repo.repo_name ? (
              <a href={`https://github.com/${repo.repo_name}/tree/${repo.branch}`} target="_blank" rel="noreferrer">
                <Badge variant="outline" className="gap-1 text-[10px] transition-colors hover:bg-muted/80 hover:text-primary">
                  <GitBranchIcon className="h-3 w-3" />
                  {repo.branch}
                </Badge>
              </a>
            ) : (
              <Badge variant="outline" className="gap-1 text-[10px]">
                <GitBranchIcon className="h-3 w-3" />
                {repo.branch}
              </Badge>
            )
          ) : null}
        </div>
      </div>

      <div className="grid min-h-0 flex-1 gap-0 md:grid-cols-[320px_minmax(0,1fr)]">
        <div className="min-h-0 border-b border-border bg-muted/25 md:border-b-0 md:border-r">
          <div className="h-full overflow-auto px-3 py-3">
            {files.length > 0 ? files.map((file) => (
              <button
                key={file.path}
                type="button"
                className={cn(
                  'mb-1.5 w-full rounded-lg border px-3 py-2 text-left text-sm transition-colors',
                  selectedFile === file.path
                    ? 'border-border bg-accent text-accent-foreground'
                    : 'border-transparent hover:border-border/60 hover:bg-accent/40',
                )}
                onClick={() => onSelectFile(file.path)}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate font-medium">{file.path}</div>
                    <div className="mt-0.5 text-xs text-muted-foreground">{file.status}</div>
                  </div>
                  <span className="rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                    {compactStatus(file.status)}
                  </span>
                </div>
              </button>
            )) : (
              <p className="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
                No changed files detected yet.
              </p>
            )}
          </div>
        </div>

        <div className="min-h-0 overflow-auto px-4 py-3">
          <div className="mb-3">
            <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              {selectedFile ? 'Selected diff' : 'Diff preview'}
            </div>
            <div className="mt-1 text-sm text-foreground">
              {selectedFile ?? 'Choose a file from the left rail'}
            </div>
          </div>

          {diffLoading ? (
            <div className="flex items-center gap-2 rounded-lg border border-dashed border-border px-4 py-3 text-sm text-muted-foreground">
              <Loading01Icon className="h-4 w-4 animate-spin" />
              Loading diff…
            </div>
          ) : diff?.diff ? (
            <div className="overflow-hidden rounded-xl border border-border">
              <div className="flex items-center gap-1.5 border-b border-border/20 bg-slate-900 px-3 py-1.5">
                <span className="h-2 w-2 rounded-full bg-red-400" />
                <span className="h-2 w-2 rounded-full bg-yellow-400" />
                <span className="h-2 w-2 rounded-full bg-green-400" />
                <span className="ml-2 text-[11px] text-slate-400">
                  git diff
                </span>
              </div>
              <pre className="whitespace-pre-wrap break-words bg-slate-950 px-4 py-4 text-[12px] leading-6 text-slate-100">{diff.diff}</pre>
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border px-4 py-8 text-sm text-muted-foreground">
              Select a changed file to inspect its diff.
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function compactStatus(status: string) {
  const value = status.trim().toLowerCase();
  if (value.startsWith('mod')) return 'mod';
  if (value.startsWith('add')) return 'add';
  if (value.startsWith('del')) return 'del';
  return value || 'file';
}
