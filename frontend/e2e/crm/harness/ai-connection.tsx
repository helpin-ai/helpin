import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { AIConnectionDialog } from '@/components/agents/AIConnectionDialog';
import '@/index.css';
if (location.search.includes('dark')) document.documentElement.classList.add('dark');
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><AIConnectionDialog workspaceId="test" scope="personal" open mode={{ kind: 'add' }} models={['openai', 'anthropic', 'openrouter', 'openai_chatgpt'].map(provider => ({ provider, selection_model: 'model', label: 'Model', tier: 'large' }))} onOpenChange={() => {}} /></TooltipProvider></QueryClientProvider>);
