import { useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";
import { FlowBuilderDrawer } from "@/components/automation/FlowBuilderDrawer";
import { useAuthStore } from "@/stores/authStore";
import type {
  Agent,
  AutomationRule,
  FlowTemplateManifest,
} from "@/lib/pmTypes";
import { installFlowBuilderFixture, fixtureFlow } from "./flow-builder-fixture";
import "@/index.css";

if (import.meta.env.MODE !== "test")
  throw new Error("This fixture is only available to browser tests");
installFlowBuilderFixture();
useAuthStore.setState({
  user: { id: "preview-user", full_name: "You" } as never,
  loading: false,
});
const agent = {
  id: "reviewer",
  name: "Code Reviewer",
  allowed_targets: ["repository"],
} as Agent;
const template = {
  key: "review_pull_requests",
  name: "Review pull requests",
} as FlowTemplateManifest;
function FlowBuilderTestHarness() {
  const [mode, setMode] = useState<"custom" | "template" | "edit" | null>(
    new URLSearchParams(location.search).get("mode") as "edit" | null,
  );
  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="flex gap-4 p-6">
        <Button onClick={() => setMode("custom")}>New flow</Button>
        <Button onClick={() => setMode("edit")}>Edit flow</Button>
        <Button onClick={() => setMode("template")}>Use template</Button>
      </div>
      {mode && (
        <FlowBuilderDrawer
          key={mode}
          workspaceId="preview-workspace"
          open
          onOpenChange={(open) => {
            if (!open) setMode(null);
          }}
          template={mode === "template" ? template : null}
          flow={mode === "edit" ? (fixtureFlow as AutomationRule) : null}
          agents={[agent]}
          workflows={[]}
          onSaved={() => {}}
        />
      )}
    </main>
  );
}
createRoot(document.getElementById("root")!).render(
  <QueryClientProvider
    client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
  >
    <TooltipProvider>
      <FlowBuilderTestHarness />
    </TooltipProvider>
  </QueryClientProvider>,
);
