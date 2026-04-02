import type { ReactNode } from 'react';
import { Moon, Sun } from 'lucide-react';
import { isModuleEnabled } from '@/lib/featureFlags';
import type { RailId, RailItem } from './types';

type SidebarRailProps = {
  railItems: RailItem[];
  activeRail: RailId;
  userEmail?: string;
  theme?: string;
  onRailSelect: (link: string) => void;
  onToggleTheme: () => void;
  accountMenu: ReactNode;
};

export function SidebarRail({
  railItems,
  activeRail,
  userEmail,
  theme,
  onRailSelect,
  onToggleTheme,
  accountMenu,
}: SidebarRailProps) {
  return (
    <div className="relative flex w-16 shrink-0 flex-col py-2 after:absolute after:top-2 after:right-0 after:bottom-2 after:w-px after:bg-border/70 after:[mask-image:linear-gradient(to_bottom,transparent,black_24px,black_calc(100%-24px),transparent)] dark:after:bg-sidebar-border">
      <div className="flex flex-1 flex-col items-center gap-1.5">
        {railItems
          .filter((item) => isModuleEnabled(item.id, userEmail))
          .map((item) => (
            <button
              key={item.id}
              type="button"
              aria-label={item.label}
              onClick={() => onRailSelect(item.defaultLink)}
              className={`flex w-12 flex-col items-center justify-center gap-1 rounded-lg px-1.5 py-1.5 transition-colors ${
                activeRail === item.id
                  ? 'text-foreground'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <div className={`relative flex h-7 w-9 items-center justify-center rounded-md transition-colors ${
                activeRail === item.id
                  ? 'bg-foreground text-background'
                  : 'hover:bg-muted/80'
              }`}>
                <item.icon className="h-4 w-4" />
                {item.indicator && (
                  <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full bg-red-500" />
                )}
                {!!item.badge && (
                  <span className="absolute -right-1.5 -top-1 flex h-3.5 min-w-[14px] items-center justify-center rounded-full bg-red-500 px-0.5 text-[9px] font-bold leading-none text-white">
                    {item.badge > 99 ? '99+' : item.badge}
                  </span>
                )}
              </div>
              <span className={`text-[10px] leading-none ${
                activeRail === item.id ? 'font-medium' : ''
              }`}>{item.label}</span>
            </button>
          ))}
      </div>

      <div className="flex flex-col items-center gap-1.5 pt-2">
        <button
          type="button"
          className="flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
          onClick={onToggleTheme}
          aria-label="Toggle theme"
        >
          <Sun className="h-3.5 w-3.5 rotate-0 scale-100 transition-transform dark:rotate-90 dark:scale-0" />
          <Moon className="absolute h-3.5 w-3.5 rotate-90 scale-0 transition-transform dark:rotate-0 dark:scale-100" />
          <span className="sr-only">{theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}</span>
        </button>
        {accountMenu}
      </div>
    </div>
  );
}
