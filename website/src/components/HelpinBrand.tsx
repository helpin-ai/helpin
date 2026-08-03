interface HelpinBrandProps {
  className?: string;
  iconClassName?: string;
  variant?: 'dark-on-light' | 'light-on-dark';
}

export function HelpinBrand({
  className = '',
  iconClassName = 'h-7 w-7',
  variant = 'dark-on-light',
}: HelpinBrandProps) {
  const icon = variant === 'light-on-dark'
    ? '/brand/helpin-icon-white.svg'
    : '/brand/helpin-icon-black.svg';

  return (
    <span
      aria-label="Helpin"
      className={`inline-flex items-center gap-2.5 text-[1.25rem] font-bold leading-none tracking-[-0.045em] ${className}`}
    >
      <img
        src={icon}
        alt=""
        aria-hidden="true"
        className={`${iconClassName} shrink-0`}
      />
      <span>Helpin</span>
    </span>
  );
}
