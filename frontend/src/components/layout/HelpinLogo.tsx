import { cn } from '@/lib/utils';

const LIGHT_LOGO_URL = 'https://assets.helpin.ai/logos/helpin-light-mode-logo.svg';
const DARK_LOGO_URL = 'https://assets.helpin.ai/logos/helpin-dark-mode-logo.svg';

interface HelpinLogoProps {
  className?: string;
  imageClassName?: string;
}

export function HelpinLogo({ className, imageClassName }: HelpinLogoProps) {
  return (
    <div className={cn('flex justify-center', className)}>
      <img
        src={LIGHT_LOGO_URL}
        alt="Helpin"
        className={cn('h-10 w-auto dark:hidden', imageClassName)}
      />
      <img
        src={DARK_LOGO_URL}
        alt="Helpin"
        className={cn('hidden h-10 w-auto dark:block', imageClassName)}
      />
    </div>
  );
}
