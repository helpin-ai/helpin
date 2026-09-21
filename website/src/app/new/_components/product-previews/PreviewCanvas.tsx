'use client';

import { useEffect, useRef, type ReactNode } from 'react';

/** Fit the shared demos like screenshots without losing their live controls. */
export function PreviewCanvas({ label, children }: { label: string; children: ReactNode }) {
  const viewport = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const frame = viewport.current;
    const content = canvas.current;
    if (!frame || !content) return;

    const fit = () => {
      const width = frame.clientWidth;
      const height = frame.clientHeight;
      // Keep narrow previews at their native text size. Wide previews get a
      // desktop-sized canvas, with a floor so short screens stay readable.
      const scale = width >= 760
        ? Math.min(1, Math.max(0.7, Math.min(width / 1200, height / 700)))
        : 1;
      frame.dataset.fitted = String(scale < 1);
      content.style.width = `${width / scale}px`;
      content.style.setProperty('--preview-height', `${height / scale}px`);
      // Add one displayed pixel of text even when the canvas is scaled down.
      content.style.setProperty('--preview-font-step', `${1 / scale}px`);
      content.style.transform = `scale(${scale})`;
    };

    fit();
    const resize = new ResizeObserver(fit);
    resize.observe(frame);
    return () => resize.disconnect();
  }, []);

  return <div ref={viewport} className="product-preview-viewport" role="region" aria-label={label} tabIndex={0}>
    <div ref={canvas} className="product-preview-canvas">{children}</div>
  </div>;
}
