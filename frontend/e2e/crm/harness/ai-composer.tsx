import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { AIProfilePicker } from '@/components/agents/AIProfilePicker';
import { DockInput } from '@/components/agents/dock/DockInput';
import '@/index.css';
if (location.search.includes('dark')) document.documentElement.classList.add('dark');
function Composer() {
  const [value, setValue] = useState('');
  const [profile, setProfile] = useState<string | null>(null);
  return <main className="mx-auto mt-32 max-w-xl bg-background text-foreground">
    <DockInput mode="conversation" value={value} onChange={setValue} onSubmit={() => setValue('')} pageContext={null} onAddMedia={() => {}} showShortcutHint={false}
      profilePicker={<AIProfilePicker workspaceId="ws" inDock compact value={profile} onChange={setProfile} />} />
  </main>;
}
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><TooltipProvider><Composer /></TooltipProvider></QueryClientProvider>);
