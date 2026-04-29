import { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { Loading01Icon } from '@/lib/icons';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { AgentRun } from '@/lib/pm-types/agents';

const ACTIVE_STATUSES = new Set(['running', 'queued', 'paused']);

export function RunsStatusPill() {
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const setRailMode = useCommandBarRunStore((s) => s.setRailMode);
  const setRailFilter = useCommandBarRunStore((s) => s.setRailFilter);
  const [hiddenByModal, setHiddenByModal] = useState(false);

  // Hide while a centered modal dialog is open (mirrors AskAgentsDock).
  useEffect(() => {
    const update = () => {
      const open = document.querySelectorAll(
        '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]',
      );
      let blocking = false;
      open.forEach((el) => {
        if (el.hasAttribute('data-side')) return;
        if (el.closest('[data-helpin-dock]')) return;
        blocking = true;
      });
      setHiddenByModal(blocking);
    };
    const obs = new MutationObserver(update);
    obs.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['data-state', 'role'],
    });
    update();
    return () => obs.disconnect();
  }, []);

  const active = useMemo(() => {
    let n = 0;
    for (const r of Object.values(runsById) as AgentRun[]) {
      if (ACTIVE_STATUSES.has(r.status)) n++;
    }
    return n;
  }, [runsById]);

  if (typeof document === 'undefined') return null;
  if (hiddenByModal) return null;
  if (active === 0) return null;

  const onClick = () => {
    setRailFilter('all');
    setRailMode('open');
  };

  return createPortal(
    <div
      data-helpin-dock="true"
      className="pointer-events-none fixed inset-x-0 bottom-[88px] z-[60] flex justify-center px-4"
    >
      <button
        type="button"
        onClick={onClick}
        title={`${active} command run${active === 1 ? '' : 's'} in progress`}
        className="pointer-events-auto inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/10 px-2.5 py-1 text-[11px] font-medium text-primary shadow-[0_1px_2px_rgba(15,23,42,0.05),0_6px_20px_-8px_rgba(15,23,42,0.18)] backdrop-blur transition hover:bg-primary/15"
      >
        <Loading01Icon className="h-3 w-3 animate-spin" />
        {active} running
      </button>
    </div>,
    document.body,
  );
}
