import { useEffect } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export default function Dashboard() {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();

  useEffect(() => {
    if (currentWorkspace?.slug) {
      navigate({ to: '/w/$slug/pm/my-work', params: { slug: currentWorkspace.slug } });
    }
  }, [currentWorkspace?.slug, navigate]);

  return null;
}
