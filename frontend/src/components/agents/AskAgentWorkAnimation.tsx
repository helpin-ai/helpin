import { useEffect, useRef } from 'react';

export function AskAgentWorkAnimation({ className = 'h-5 w-5' }: { className?: string }) {
  const containerRef = useRef<HTMLSpanElement | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    if (typeof navigator !== 'undefined' && /jsdom/i.test(navigator.userAgent)) return;

    let cancelled = false;
    let animation: { destroy: () => void } | null = null;
    void import('lottie-web').then(({ default: lottie }) => {
      if (cancelled || !containerRef.current) return;
      const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
      const instance = lottie.loadAnimation({
        container: containerRef.current,
        renderer: 'svg',
        loop: !reducedMotion,
        autoplay: !reducedMotion,
        path: '/assets/agents/loader.json',
        rendererSettings: { progressiveLoad: true },
      });
      animation = instance;
      if (reducedMotion) instance.goToAndStop(0, true);
    });
    return () => {
      cancelled = true;
      animation?.destroy();
    };
  }, []);

  return <span ref={containerRef} className={`${className} shrink-0`} aria-hidden="true" data-agent-work-loader />;
}
