import { useEffect, type ReactNode } from 'react';
import { useLocation } from '@tanstack/react-router';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { SETTINGS_ROUTE_SECTIONS } from '@/lib/settingsSections';
import { rememberSetting, revealSettingOption } from '@/lib/settingsDiscovery';
import { QuietPageViewport } from '@/components/design-system/quiet';

export function SettingsRouteViewport({ children }: { children: ReactNode }) {
  const { pathname, hash } = useLocation();
  const workspaceId = useWorkspaceStore(state => state.currentWorkspace?.id);
  const userId = useAuthStore(state => state.user?.id);
  useEffect(() => {
    const id = pathname.replace(/\/$/, '').split('/').at(-1);
    const section = SETTINGS_ROUTE_SECTIONS.find(section => section.id === id);
    if (section && workspaceId && userId) rememberSetting(`${userId}:${workspaceId}`, section.id);
    const optionId = hash.replace(/^#/, '');
    if (!section?.options?.some(option => option.id === optionId)) return;
    let done = false;
    const reveal = () => { if (!done) done = revealSettingOption(optionId); if (done) observer.disconnect(); };
    const observer = new MutationObserver(reveal);
    observer.observe(document.body, { childList: true, subtree: true });
    const frame = requestAnimationFrame(reveal);
    const timeout = window.setTimeout(() => observer.disconnect(), 20000);
    return () => { observer.disconnect(); cancelAnimationFrame(frame); window.clearTimeout(timeout); };
  }, [pathname, hash, workspaceId, userId]);
  return (
    <QuietPageViewport className="pb-32 md:pb-32">
      {children}
    </QuietPageViewport>
  );
}
