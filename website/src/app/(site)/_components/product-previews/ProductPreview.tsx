'use client';

import dynamic from 'next/dynamic';
import { PreviewCanvas } from './PreviewCanvas';
import './product-preview.css';

export type ProductPreviewName = 'inbox' | 'meetings' | 'projects' | 'crm' | 'knowledge' | 'ask-agent' | 'agents';

function LoadingPreview() {
  return <div className="product-preview-loading" role="status">Loading preview…</div>;
}

// Shared entry point for embedded product demos. These are the same components
// used on the product pages; keep their interactions and stories in those files.
// Load requested demos on the client; callers can preload nearby scenes. This
// keeps accessibility IDs stable while the loading frame reserves its space.
const PREVIEWS = {
  agents: dynamic(() => import('../agent-directory/AgentDirectoryPreview').then(module => module.AgentDirectoryPreview), { loading: LoadingPreview, ssr: false }),
  'ask-agent': dynamic(() => import('../ask-agent/AskAgentWorkspace').then(module => module.AskAgentWorkspace), { loading: LoadingPreview, ssr: false }),
  inbox: dynamic(() => import('../../products/customer-support/support-workspace').then(module => module.SupportWorkspace), { loading: LoadingPreview, ssr: false }),
  meetings: dynamic(() => import('../../products/meetings/meeting-workspace').then(module => module.MeetingWorkspace), { loading: LoadingPreview, ssr: false }),
  projects: dynamic(() => import('../../products/projects/project-hero').then(module => module.ProjectHero), { loading: LoadingPreview, ssr: false }),
  crm: dynamic(() => import('../../products/crm/crm-workspace').then(module => module.CRMWorkspace), { loading: LoadingPreview, ssr: false }),
  knowledge: dynamic(() => import('../../products/knowledge/knowledge-previews').then(module => module.KnowledgeWorkspace), { loading: LoadingPreview, ssr: false }),
};

const LABELS: Record<ProductPreviewName, string> = {
  agents: 'Agents', inbox: 'Inbox', meetings: 'Meetings', projects: 'Projects', crm: 'CRM', knowledge: 'Knowledge', 'ask-agent': 'Ask Agent',
};

/** A bounded, responsive demo for product tabs and other embedded surfaces. */
export function ProductPreview({ product, theme = 'dark' }: { product: ProductPreviewName; theme?: 'light' | 'dark' }) {
  const Preview = PREVIEWS[product];
  return <div className="product-preview" data-product={product} data-theme={theme}>
    <PreviewCanvas label={`${LABELS[product]} interactive product preview`}>
      {product === 'knowledge'
        ? <PREVIEWS.knowledge apiHref="/products/knowledge#knowledge-api" />
        : product === 'projects' ? <PREVIEWS.projects theme={theme} /> : <Preview />}
    </PreviewCanvas>
  </div>;
}
