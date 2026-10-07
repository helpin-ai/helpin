import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { AISettingsPage } from "@/pages/settings/AISettingsPage";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import "@/index.css";
useWorkspaceStore.setState({
  currentWorkspace: {
    id: "ws",
    name: "Example workspace",
    slug: "example",
  } as never,
});
createRoot(document.getElementById("root")!).render(
  <QueryClientProvider
    client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
  >
    <TooltipProvider>
      <main className="mx-auto max-w-5xl p-6">
        <AISettingsPage />
      </main>
      <Toaster />
    </TooltipProvider>
  </QueryClientProvider>,
);
