'use client';

import { useEffect, useRef, useState } from 'react';
import { ProductPreview, type ProductPreviewName } from '../_components/product-previews';

// Mounts a live product demo only when it nears the viewport. The stage reserves the
// demo's height from the start, so loading it never shifts the page.
export function LazyPreview({ product, label }: { product: ProductPreviewName; label: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return; }
    const observer = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      setVisible(true);
      observer.disconnect();
    }, { rootMargin: '600px 0px' });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return (
    <figure ref={ref} className="cmp-preview">
      {visible ? <ProductPreview product={product} theme="light" /> : <div className="cmp-preview-placeholder" aria-hidden="true" />}
      <figcaption>{label}</figcaption>
    </figure>
  );
}
