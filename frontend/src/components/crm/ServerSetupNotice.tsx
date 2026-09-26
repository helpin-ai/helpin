import { SetupSettingsLink } from '@/components/setup/CapabilityActions';
import { cn } from '@/lib/utils';

/**
 * Explains that a feature needs server configuration. Server admins get a link
 * to Settings → System status, which names the exact settings; everyone else
 * is told who can fix it. Pass `id` so disabled controls can reference the
 * reason with `aria-describedby`.
 */
export function ServerSetupNotice({
  id,
  title,
  slug,
  isServerAdmin,
  className,
}: {
  id: string;
  title: string;
  slug: string;
  isServerAdmin: boolean;
  className?: string;
}) {
  return (
    <div role="note" className={cn('max-w-[680px] border-l border-quiet-divider-strong py-1 pl-4', className)}>
      <p id={id} className="text-sm font-medium leading-[1.6] text-quiet-text-primary">{title}</p>
      {isServerAdmin ? (
        <div className="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-[12.5px] leading-5 text-quiet-text-tertiary">
          <span>System status lists the server settings it needs.</span>
          <SetupSettingsLink slug={slug} path="settings/system-status">Open System status</SetupSettingsLink>
        </div>
      ) : (
        <p className="mt-1 text-[12.5px] leading-5 text-quiet-text-tertiary">Ask your server admin to set it up.</p>
      )}
    </div>
  );
}
