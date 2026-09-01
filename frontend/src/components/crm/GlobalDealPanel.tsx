import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { DealDetailPage } from '@/pages/crm/DealDetail';
import {
  closeDealRoute,
  getActiveDealRoute,
  type DealOverlayLocationLike,
} from '@/components/crm/deal-detail/dealRouteNavigation';
import { useDealPanelStore } from '@/stores/dealPanelStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';

interface GlobalDealPanelProps {
  workspaceId: string;
}

export function GlobalDealPanel({ workspaceId }: GlobalDealPanelProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const overlayLocation = location as DealOverlayLocationLike;
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceSlug = workspace?.slug ?? '';
  const contextualDealId = useDealPanelStore((state) => state.dealId);
  const closeContextualDeal = useDealPanelStore((state) => state.close);
  const activeDealRoute = useMemo(() => getActiveDealRoute(overlayLocation), [overlayLocation]);
  const activeDealId = activeDealRoute?.dealId ?? contextualDealId;
  const beforeCloseRef = useRef<(() => Promise<void>) | null>(null);
  const closingRef = useRef(false);

  useEffect(() => {
    if (activeDealRoute && contextualDealId) closeContextualDeal();
  }, [activeDealRoute, closeContextualDeal, contextualDealId]);

  const handleClose = useCallback(async () => {
    if (!workspaceSlug || closingRef.current) return;
    closingRef.current = true;
    try {
      await beforeCloseRef.current?.();
      await closeDealRoute(navigate as never, overlayLocation, workspaceSlug);
    } finally {
      closingRef.current = false;
    }
  }, [navigate, overlayLocation, workspaceSlug]);

  if (!workspaceId) return null;

  return (
    <Sheet
      open={!!activeDealId}
      onOpenChange={(open) => {
        if (!open) void handleClose();
      }}
    >
      <SheetContent
        side="right"
        className="h-dvh overflow-hidden p-0 data-[side=right]:w-screen data-[side=right]:!max-w-none md:data-[side=right]:w-[80vw] md:data-[side=right]:!max-w-[1200px]"
		showCloseButton={false}
      >
        <SheetTitle className="sr-only">Deal details</SheetTitle>
        {activeDealId ? (
          <DealDetailPage
            key={activeDealId}
            dealId={activeDealId}
            onRequestClose={handleClose}
            registerBeforeClose={(handler) => { beforeCloseRef.current = handler; }}
          />
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
