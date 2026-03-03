import { SidebarTrigger } from '@/components/ui/sidebar';
import { QuarterSelector } from '@/components/quarter/QuarterSelector';

export function Header() {
  return (
    <header className="h-14 border-b bg-background flex items-center px-4 gap-4">
      <SidebarTrigger className="-ml-1" />
      <div className="flex-1" />
      <QuarterSelector />
    </header>
  );
}
