import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { EmailsPage } from "@/pages/crm/Emails";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { useAuthStore } from "@/stores/authStore";
import type { Workspace, User } from "@/lib/types";
import "@/index.css";
useWorkspaceStore.setState({
  currentWorkspace: {
    id: "ws-email",
    slug: "email-test",
    name: "ContentStudio",
  } as Workspace,
});
useAuthStore.setState({ user: { id: "owner" } as User });
if (new URLSearchParams(location.search).has("dark"))
  document.documentElement.classList.add("dark");
const client = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});
createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={client}>
    <TooltipProvider>
      <div className="h-screen bg-background text-foreground">
        <EmailsPage />
      </div>
      <Toaster />
    </TooltipProvider>
  </QueryClientProvider>,
);
