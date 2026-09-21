'use client';

import dynamic from 'next/dynamic';
import './product-preview.css';

export type ProductPreviewName = 'inbox' | 'meetings' | 'projects' | 'crm' | 'knowledge';

function LoadingPreview() {
  return <div className="product-preview-loading" role="status">Loading preview…</div>;
}

// Shared entry point for embedded product demos. These are the same components
// used on the product pages; keep their interactions and stories in those files.
// Load only the selected demo on the client. This keeps generated accessibility
// IDs stable across the dynamic boundary; the loading frame reserves its space.
const PREVIEWS = {
  inbox: dynamic(() => import('../../products/customer-support/support-workspace').then(module => module.SupportWorkspace), { loading: LoadingPreview, ssr: false }),
  meetings: dynamic(() => import('../../products/meetings/meeting-workspace').then(module => module.MeetingWorkspace), { loading: LoadingPreview, ssr: false }),
  projects: dynamic(() => import('../../products/projects/project-hero').then(module => module.ProjectHero), { loading: LoadingPreview, ssr: false }),
  crm: dynamic(() => import('../../products/crm/crm-workspace').then(module => module.CRMWorkspace), { loading: LoadingPreview, ssr: false }),
  knowledge: dynamic(() => import('../../products/knowledge/knowledge-previews').then(module => module.KnowledgeWorkspace), { loading: LoadingPreview, ssr: false }),
};

const LABELS: Record<ProductPreviewName, string> = {
  inbox: 'Inbox', meetings: 'Meetings', projects: 'Projects', crm: 'CRM', knowledge: 'Knowledge',
};

/** A bounded, responsive demo for product tabs and other embedded surfaces. */
export function ProductPreview({ product, theme = 'dark' }: { product: ProductPreviewName; theme?: 'light' | 'dark' }) {
  const Preview = PREVIEWS[product];
  return <div className="product-preview" data-product={product} data-theme={theme}>
    <div className="product-preview-viewport" role="region" aria-label={`${LABELS[product]} interactive product preview`} tabIndex={0}>
      {product === 'knowledge'
        ? <PREVIEWS.knowledge apiHref="/new/products/knowledge#knowledge-api" />
        : product === 'projects' ? <PREVIEWS.projects theme={theme} /> : <Preview />}
    </div>
  </div>;
}
