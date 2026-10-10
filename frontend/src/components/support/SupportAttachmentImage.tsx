import { useEffect, useRef, useState } from 'react';
import { Image01Icon, Loading01Icon } from '@/lib/icons';
import type { SupportAttachmentPayload } from '@/lib/pmTypes';

interface SupportAttachmentImageProps {
  attachment: SupportAttachmentPayload;
  className: string;
  compact?: boolean;
  loadFallback?: (id: string) => Promise<string>;
}

export function SupportAttachmentImage({ attachment, className, compact = false, loadFallback }: SupportAttachmentImageProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const [nearViewport, setNearViewport] = useState(() => !compact || typeof IntersectionObserver === 'undefined');
  const [status, setStatus] = useState<'loading' | 'ready' | 'failed'>('loading');
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (nearViewport || !containerRef.current) return;
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        setNearViewport(true);
        observer.disconnect();
      }
    }, { rootMargin: '240px' });
    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [nearViewport]);

  useEffect(() => {
    const image = imageRef.current;
    if (!nearViewport || !image) return;
    let active = true;
    let recovering = false;
    let loadedSuccessfully = false;
    let timer: number;
    setStatus('loading');

    function fail() {
      window.clearTimeout(timer);
      if (active && !loadedSuccessfully) setStatus('failed');
    }
    function loaded() {
      window.clearTimeout(timer);
      loadedSuccessfully = true;
      if (active) setStatus('ready');
    }
    function recover() {
      window.clearTimeout(timer);
      if (!loadFallback || recovering) { fail(); return; }
      recovering = true;
      void loadFallback(attachment.id).then(url => {
        if (!active || loadedSuccessfully) return;
        image!.src = url;
        timer = window.setTimeout(fail, 8_000);
      }).catch(fail);
    }

    image.addEventListener('load', loaded);
    image.addEventListener('error', recover);
    image.src = attachment.url;
    timer = window.setTimeout(recover, 8_000);
    return () => {
      active = false;
      window.clearTimeout(timer);
      image.removeEventListener('load', loaded);
      image.removeEventListener('error', recover);
    };
  }, [nearViewport, attachment.id, attachment.url, loadFallback, attempt]);

  return (
    <div ref={containerRef} className={`relative flex items-center justify-center ${className}`} onClick={compact ? undefined : event => event.stopPropagation()}>
      <img ref={imageRef} alt={attachment.file_name} decoding="async" fetchPriority={compact ? 'low' : undefined} className={`h-full w-full ${compact ? 'object-cover' : 'object-contain'} ${status === 'ready' ? '' : 'invisible'}`} />
      {status !== 'ready' && (
        <div role="status" className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-center text-xs" title={status === 'failed' ? 'Could not load image' : 'Loading image'}>
          {status === 'loading'
            ? <Loading01Icon className="h-4 w-4 animate-spin" />
            : <Image01Icon className="h-4 w-4" />}
          {!compact && (status === 'failed'
            ? <><span>Could not load image</span><button type="button" className="rounded border border-current/30 px-3 py-1 hover:bg-current/10" onClick={() => setAttempt(value => value + 1)}>Retry</button></>
            : <span>Loading image…</span>)}
        </div>
      )}
    </div>
  );
}
