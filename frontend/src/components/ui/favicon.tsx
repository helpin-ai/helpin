import { useEffect, useMemo, useState } from 'react';
import { cn, getInitials } from '@/lib/utils';
import { getGoogleFaviconUrl, normalizeFaviconHost } from '@/lib/favicon';

interface FaviconProps {
  url?: string | null;
  src?: string | null;
  name?: string | null;
  size?: number;
  className?: string;
  imageClassName?: string;
  fallbackClassName?: string;
}

export function Favicon({
  url,
  src,
  name,
  size = 128,
  className,
  imageClassName,
  fallbackClassName,
}: FaviconProps) {
  const normalizedHost = useMemo(() => normalizeFaviconHost(url), [url]);
  const computedSrc = useMemo(() => src?.trim() || getGoogleFaviconUrl(normalizedHost, size), [normalizedHost, size, src]);
  const [imageFailed, setImageFailed] = useState(false);

  useEffect(() => {
    setImageFailed(false);
  }, [computedSrc]);

  const fallbackLabel = getInitials(name || normalizedHost || url);

  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden border border-border/70 bg-muted/30 text-[9px] font-semibold text-muted-foreground',
        className,
      )}
    >
      {!imageFailed && computedSrc ? (
        <img
          src={computedSrc}
          alt=""
          className={cn('h-full w-full object-contain', imageClassName)}
          loading="lazy"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <span className={cn('select-none uppercase', fallbackClassName)}>
          {fallbackLabel}
        </span>
      )}
    </div>
  );
}
