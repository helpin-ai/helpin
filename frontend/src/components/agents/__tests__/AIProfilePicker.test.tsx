// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AIProfilePicker } from "../AIProfilePicker";
import { AIConnectionPicker } from "../AIConnectionPicker";
import {
  aiProfileService,
  type AIProfile,
} from "@/lib/services/aiProfileService";
import { useWorkspaceStore } from "@/stores/workspaceStore";

vi.mock("@/lib/services/aiProfileService", () => ({
  aiProfileService: { list: vi.fn() },
}));
(
  globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let client: QueryClient;
const profiles: AIProfile[] = ["personal", "workspace"].map((scope) => ({
  id: scope,
  workspace_id: "ws",
  user_id: scope === "personal" ? "user" : null,
  scope: scope as AIProfile["scope"],
  name: `${scope} profile`,
  revision: 1,
  primary: {
    connection_id: scope,
    model: { provider: "openai", model: "custom-model", controls: {} },
  },
  fallback: null,
}));
beforeEach(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  HTMLElement.prototype.scrollIntoView = vi.fn();
  useWorkspaceStore.setState({ currentWorkspace: null });
  const container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.mocked(aiProfileService.list).mockResolvedValue({
    data: profiles,
    error: null,
  });
});
afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function openPicker() {
  for (
    let i = 0;
    i < 20 && !document.querySelector('button[aria-haspopup="dialog"]');
    i++
  ) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }
  const trigger = document.querySelector<HTMLButtonElement>(
    'button[aria-haspopup="dialog"]',
  );
  expect(trigger).not.toBeNull();
  await act(async () => trigger!.click());
}

it("offers only shared profiles for agent defaults", async () => {
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <AIProfilePicker workspaceId="ws" sharedOnly onChange={() => {}} />
      </QueryClientProvider>,
    ),
  );
  await openPicker();
  expect(
    document.querySelector('[cmdk-item][data-value="personal"]'),
  ).toBeNull();
  expect(
    document.querySelector('[cmdk-item][data-value="workspace"]'),
  ).not.toBeNull();
  expect(document.body.textContent).toContain("Workspace default");
});

it("replaces legacy launch fields with one explicit profile selection", async () => {
  const onChange = vi.fn();
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <AIConnectionPicker
          workspaceId="ws"
          value={{ model_connection_id: "old", model_name: "old-model" }}
          onChange={onChange}
        />
      </QueryClientProvider>,
    ),
  );
  await openPicker();
  await act(async () =>
    document
      .querySelector<HTMLElement>('[cmdk-item][data-value="personal"]')!
      .click(),
  );
  expect(onChange).toHaveBeenCalledWith({ ai_profile_id: "personal" });
});
