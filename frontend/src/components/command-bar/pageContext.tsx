import { createContext, useCallback, useContext, useEffect, useId, useMemo, useState } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CommandBarPageContext } from '@/lib/pmTypes';

interface RegisteredPageContext {
  id: string;
  priority: number;
  context: CommandBarPageContext;
}

interface PageContextValue {
  pageContext: CommandBarPageContext | null;
  register: (entry: RegisteredPageContext) => void;
  unregister: (id: string) => void;
}

const PageContext = createContext<PageContextValue | null>(null);

export function PageContextProvider({ children }: { children: React.ReactNode }) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [entries, setEntries] = useState<RegisteredPageContext[]>([]);

  const fallback = useMemo(
    () => workspace
      ? ({
        entity_type: 'workspace',
        entity_id: workspace.id,
        display_title: workspace.name,
      } satisfies CommandBarPageContext)
      : null,
    [workspace?.id, workspace?.name],
  );

  const pageContext = useMemo(() => {
    if (entries.length === 0) return fallback;
    return [...entries].sort((a, b) => b.priority - a.priority)[0]?.context ?? fallback;
  }, [entries, fallback]);

  const register = useCallback((entry: RegisteredPageContext) => {
    setEntries((current) => {
      const next = current.filter((item) => item.id !== entry.id);
      next.push(entry);
      return next;
    });
  }, []);

  const unregister = useCallback((id: string) => {
    setEntries((current) => current.filter((item) => item.id !== id));
  }, []);

  const value = useMemo<PageContextValue>(
    () => ({
      pageContext,
      register,
      unregister,
    }),
    [pageContext, register, unregister],
  );

  return <PageContext.Provider value={value}>{children}</PageContext.Provider>;
}

export function usePageContext() {
  return useContext(PageContext)?.pageContext ?? null;
}

export function useRegisterPageContext(context: CommandBarPageContext | null, priority = 10) {
  const id = useId();
  const value = useContext(PageContext);
  const register = value?.register;
  const unregister = value?.unregister;
  const contextKey = context ? stablePageContextKey(context) : '';

  useEffect(() => {
    if (!register || !unregister || !context) return;
    register({ id, priority, context });
    return () => unregister(id);
  }, [contextKey, id, priority, register, unregister]);
}

function stablePageContextKey(context: CommandBarPageContext): string {
  return stableJSONStringify(context);
}

function stableJSONStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableJSONStringify(item)).join(',')}]`;
  }
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableJSONStringify(record[key])}`)
      .join(',')}}`;
  }
  return JSON.stringify(value);
}
