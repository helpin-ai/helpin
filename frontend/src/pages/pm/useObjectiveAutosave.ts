import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { UpdateObjectiveRequest } from '@/lib/pmTypes';

/** One serialized queue for debounced edits and navigation-time saves. */
export function useObjectiveAutosave<Patch extends object = UpdateObjectiveRequest>({
  save,
  pendingUploads,
  debounceMs = 650,
}: {
  save: (patch: Partial<Patch>) => Promise<void>;
  debounceMs?: number | null;
  pendingUploads: number;
}) {
  const pendingPatchRef = useRef<Partial<Patch>>({});
  const saveRef = useRef(save);
  const uploadsRef = useRef(pendingUploads);
  const inFlight = useRef<Promise<void> | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mounted = useRef(true);
  const [version, setVersion] = useState(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasPending, setHasPending] = useState(false);
  useLayoutEffect(() => { saveRef.current = save; uploadsRef.current = pendingUploads; }, [save, pendingUploads]);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; if (timer.current) clearTimeout(timer.current); };
  }, []);

  const flush = useCallback(async () => {
    if (timer.current) clearTimeout(timer.current);
    if (inFlight.current) return inFlight.current;
    if (uploadsRef.current === 0 && Object.keys(pendingPatchRef.current).length === 0) { setError(null); return; }
    const run = async () => {
      if (mounted.current) { setSaving(true); setError(null); }
      try {
        if (uploadsRef.current > 0) throw new Error('Wait for uploads to finish before leaving.');
        while (Object.keys(pendingPatchRef.current).length > 0) {
          if (uploadsRef.current > 0) throw new Error('Wait for uploads to finish before leaving.');
          const patch = pendingPatchRef.current;
          pendingPatchRef.current = {};
          try {
            await saveRef.current(patch);
          } catch (cause) {
            pendingPatchRef.current = { ...patch, ...pendingPatchRef.current };
            throw cause;
          }
        }
      } catch (cause) {
        if (mounted.current) setError(cause instanceof Error ? cause.message : 'Failed to save objective');
        throw cause;
      } finally {
        if (mounted.current) { setSaving(false); setHasPending(Object.keys(pendingPatchRef.current).length > 0); setVersion(current => current + 1); }
      }
    };
    inFlight.current = run();
    try { await inFlight.current; } finally { inFlight.current = null; }
  }, []);

  const queuePatch = useCallback((patch: Partial<Patch>) => {
    pendingPatchRef.current = { ...pendingPatchRef.current, ...patch };
    setHasPending(true);
    setVersion(current => current + 1);
  }, []);

  useEffect(() => {
    if (debounceMs === null || saving || error || pendingUploads > 0 || Object.keys(pendingPatchRef.current).length === 0) return;
    timer.current = setTimeout(() => { void flush().catch(() => undefined); }, debounceMs);
    return () => { if (timer.current) clearTimeout(timer.current); };
  }, [version, saving, error, pendingUploads, flush, debounceMs]);

  return { queuePatch, pendingPatchRef, flush, saving, error,
    dirty: saving || pendingUploads > 0 || hasPending };
}
