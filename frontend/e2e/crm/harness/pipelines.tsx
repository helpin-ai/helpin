import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { PipelineSettings } from "@/components/crm/PipelineSettings";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import type { Workspace } from "@/lib/types";
import "@/index.css";

useWorkspaceStore.setState({
  currentWorkspace: {
    id: "ws-pipelines",
    slug: "pipelines-test",
    name: "Pipeline test",
  } as Workspace,
});
if (new URLSearchParams(window.location.search).has("dark"))
  document.documentElement.classList.add("dark");
const client = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});
createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={client}>
    <TooltipProvider>
      <main className="mx-auto max-w-6xl space-y-6 p-4 text-foreground sm:p-8">
        <header>
          <h1 className="text-2xl font-semibold">Deal pipelines</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Shape your sales process and manage the stages deals move through.
          </p>
        </header>
        <PipelineSettings />
      </main>
      <Toaster />
    </TooltipProvider>
  </QueryClientProvider>,
);
