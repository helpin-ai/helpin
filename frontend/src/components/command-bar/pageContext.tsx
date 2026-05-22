import { createContext, useCallback, useContext, useEffect, useId, useMemo, useState } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CommandBarPageContext } from '@/lib/pmTypes';

export interface PageContextScopeOption {
  key: string;
  label: string;
  description?: string;
  context: CommandBarPageContext;
}

interface RegisteredPageContext {
  id: string;
  priority: number;
  context: CommandBarPageContext;
  scopeOptions?: PageContextScopeOption[];
  defaultScopeKey?: string;
}

interface PageContextValue {
  pageContext: CommandBarPageContext | null;
  scopeOptions: PageContextScopeOption[];
  activeScopeKey: string | null;
  setActiveScopeKey: (key: string) => void;
  register: (entry: RegisteredPageContext) => void;
  unregister: (id: string) => void;
}

const PageContext = createContext<PageContextValue | null>(null);

export function PageContextProvider({ children }: { children: React.ReactNode }) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [entries, setEntries] = useState<RegisteredPageContext[]>([]);
  const [selectedScopeKeys, setSelectedScopeKeys] = useState<Record<string, string>>({});

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

  const activeEntry = useMemo(() => {
    if (entries.length === 0) return null;
    return [...entries].sort((a, b) => b.priority - a.priority)[0] ?? null;
  }, [entries]);

  const activeScopes = useMemo<PageContextScopeOption[]>(() => {
    if (!activeEntry) return [];
    const scopes = activeEntry.scopeOptions?.filter((scope) => scope.key.trim() && scope.context) ?? [];
    if (scopes.length > 0) return scopes;
    return [{
      key: 'current',
      label: activeEntry.context.display_title || activeEntry.context.entity_type,
      context: activeEntry.context,
    }];
  }, [activeEntry]);

  const activeScopeKey = useMemo(() => {
    if (!activeEntry || activeScopes.length === 0) return null;
    const selected = selectedScopeKeys[activeEntry.id];
    if (selected && activeScopes.some((scope) => scope.key === selected)) return selected;
    if (activeEntry.defaultScopeKey && activeScopes.some((scope) => scope.key === activeEntry.defaultScopeKey)) {
      return activeEntry.defaultScopeKey;
    }
    return activeScopes[0]?.key ?? null;
  }, [activeEntry, activeScopes, selectedScopeKeys]);

  const pageContext = useMemo(() => {
    if (!activeEntry) return fallback;
    return activeScopes.find((scope) => scope.key === activeScopeKey)?.context ?? activeEntry.context ?? fallback;
  }, [activeEntry, activeScopeKey, activeScopes, fallback]);

  const setActiveScopeKey = useCallback((key: string) => {
    if (!activeEntry) return;
    const trimmed = key.trim();
    if (!trimmed || !activeScopes.some((scope) => scope.key === trimmed)) return;
    setSelectedScopeKeys((current) => ({ ...current, [activeEntry.id]: trimmed }));
  }, [activeEntry, activeScopes]);

  useEffect(() => {
    setSelectedScopeKeys((current) => {
      const activeIDs = new Set(entries.map((entry) => entry.id));
      const next = Object.fromEntries(Object.entries(current).filter(([id]) => activeIDs.has(id)));
      return Object.keys(next).length === Object.keys(current).length ? current : next;
    });
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
      scopeOptions: activeScopes,
      activeScopeKey,
      setActiveScopeKey,
      register,
      unregister,
    }),
    [activeScopeKey, activeScopes, pageContext, register, setActiveScopeKey, unregister],
  );

  return <PageContext.Provider value={value}>{children}</PageContext.Provider>;
}

export function usePageContext() {
  return useContext(PageContext)?.pageContext ?? null;
}

export function usePageContextState() {
  const value = useContext(PageContext);
  return {
    pageContext: value?.pageContext ?? null,
    scopeOptions: value?.scopeOptions ?? [],
    activeScopeKey: value?.activeScopeKey ?? null,
    setActiveScopeKey: value?.setActiveScopeKey ?? (() => {}),
  };
}

export function useRegisterPageContext(context: CommandBarPageContext | null, priority = 10, options?: { scopeOptions?: PageContextScopeOption[]; defaultScopeKey?: string }) {
  const id = useId();
  const value = useContext(PageContext);
  const register = value?.register;
  const unregister = value?.unregister;
  const contextKey = context ? stableJSONStringify({
    context,
    scopeOptions: options?.scopeOptions,
    defaultScopeKey: options?.defaultScopeKey,
  }) : '';

  useEffect(() => {
    if (!register || !unregister || !context) return;
    register({ id, priority, context, scopeOptions: options?.scopeOptions, defaultScopeKey: options?.defaultScopeKey });
    return () => unregister(id);
  }, [contextKey, id, priority, register, unregister]);
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
