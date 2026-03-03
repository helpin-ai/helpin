import { Menu } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { QuarterSelector } from '@/components/quarter/QuarterSelector';

interface HeaderProps {
  onMenuClick: () => void;
}

export function Header({ onMenuClick }: HeaderProps) {
  return (
    <header className="h-14 border-b bg-background flex items-center px-4 gap-4">
      <Button variant="ghost" size="icon" className="lg:hidden" onClick={onMenuClick}>
        <Menu className="h-5 w-5" />
      </Button>
      <div className="flex-1" />
      <QuarterSelector />
    </header>
  );
}
