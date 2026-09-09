import { useEffect, useState } from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

export function forwardingRelativeTime(value: string, now = Date.now()) {
  const seconds = Math.max(0, (now - Date.parse(value)) / 1000);
  if (!Number.isFinite(seconds)) return null;
  if (seconds < 60) return 'just now';
  const [unit, size] = seconds < 3600 ? ['minute', 60] as const
    : seconds < 86400 ? ['hour', 3600] as const
    : seconds < 2592000 ? ['day', 86400] as const
    : seconds < 31536000 ? ['month', 2592000] as const : ['year', 31536000] as const;
  return new Intl.RelativeTimeFormat('en', { style: 'short' }).format(-Math.floor(seconds / size), unit);
}
export function ForwardingRelativeTime({ value }: { value: string }) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 30000);
    return () => clearInterval(timer);
  }, []);
  const label = forwardingRelativeTime(value, now);
  if (!label) return null;
  return <Tooltip><TooltipTrigger asChild><time tabIndex={0} dateTime={value}>{label}</time></TooltipTrigger><TooltipContent>{new Date(value).toLocaleString()}</TooltipContent></Tooltip>;
}
