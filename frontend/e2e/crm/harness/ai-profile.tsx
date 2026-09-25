import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { AIProfileEditor } from '@/components/agents/AIProfileEditor';
import '@/index.css';
if (location.search.includes('dark')) document.documentElement.classList.add('dark');
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><AIProfileEditor workspaceId="test" scope="personal" connections={[{ id: 'primary', name: 'My OpenAI', provider: 'openai', scope: 'personal', user_id: 'me', status: 'connected' }, { id: 'backup', name: 'My Anthropic', provider: 'anthropic', scope: 'personal', user_id: 'me', status: 'connected' }]} onClose={() => {}} /></TooltipProvider></QueryClientProvider>);
