'use client';

import dynamic from 'next/dynamic';
import { useEffect, useRef, useState } from 'react';
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

const DocsWorkflow = dynamic(() => import('../WorkScene').then(module => module.WorkScene), { loading: LoadingPreview, ssr: false });

const LABELS: Record<ProductPreviewName, string> = {
  agents: 'Agents', inbox: 'Inbox', meetings: 'Meetings', projects: 'Projects', crm: 'CRM', knowledge: 'Knowledge', 'ask-agent': 'Helpin AI',
};

/** A bounded, responsive demo for product tabs and other embedded surfaces. */
export function ProductPreview({ product, theme = 'dark', workflow = false }: { product: ProductPreviewName; theme?: 'light' | 'dark'; workflow?: boolean }) {
  const [near, setNear] = useState(false);
  const container = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) { setNear(true); observer.disconnect(); }
    }, { rootMargin: '240px' });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const Preview = PREVIEWS[product];
  return <div ref={container} className="product-preview" data-product={product} data-theme={theme} data-workflow={workflow}>
    <PreviewCanvas label={`${LABELS[product]} interactive product preview`}>
      {!near ? <LoadingPreview /> : workflow && product === 'knowledge' ? <DocsWorkflow variant="docs" /> : workflow && product === 'crm' ? <PREVIEWS.crm mode="account" /> : product === 'knowledge'
        ? <PREVIEWS.knowledge apiHref="/products/knowledge#knowledge-api" />
        : product === 'projects' ? <PREVIEWS.projects theme={theme} /> : <Preview />}
    </PreviewCanvas>
  </div>;
}
