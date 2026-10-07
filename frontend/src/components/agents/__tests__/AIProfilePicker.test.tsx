// @vitest-environment jsdom
import { queryKeys } from "@/lib/queryKeys"
import { TooltipProvider } from "@/components/ui/tooltip"
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AIProfilePicker } from "../AIProfilePicker";
import { AIConnectionPicker } from "../AIConnectionPicker";
import { EpicDeliveryProfileSelect } from '@/components/pm/EpicDeliveryProfileSelect';
import {
  aiProfileService,
  type AIProfile,
} from "@/lib/services/aiProfileService";
import { useWorkspaceStore } from "@/stores/workspaceStore";

vi.mock("@/lib/services/aiProfileService", () => ({
  aiProfileService: { list: vi.fn(), settings: vi.fn() },
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
  vi.mocked(aiProfileService.settings).mockResolvedValue({data: { default_profile_id: "workspace" }, error: null});
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
      <QueryClientProvider client={client}><TooltipProvider>
        <AIProfilePicker workspaceId="ws" sharedOnly onChange={() => {}} />
      </TooltipProvider></QueryClientProvider>,
    ),
  );
  await openPicker();
  expect(
    document.querySelector('[cmdk-item][data-value="personal"]'),
  ).toBeNull();
  expect(
    document.querySelector('[cmdk-item][data-value="workspace"]'),
  ).not.toBeNull();
  const row = document.querySelector('[cmdk-item][data-value="workspace"]');
  expect(row?.textContent).toContain("workspace profile");
  expect(row?.textContent).toContain("Default");
  expect(document.querySelector('[cmdk-item][data-value="default"]')).toBeNull();
});

it("replaces legacy launch fields with one explicit profile selection", async () => {
  const onChange = vi.fn();
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}><TooltipProvider>
        <AIConnectionPicker
          workspaceId="ws"
          value={{ model_connection_id: "old", model_name: "old-model" }}
          onChange={onChange}
        />
      </TooltipProvider></QueryClientProvider>,
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

it("keeps the personal profile menu owned by and above the Ask Agent dock", async () => {
  const onChange = vi.fn();
  await act(async () => root.render(
    <QueryClientProvider client={client}><TooltipProvider>
      <AIConnectionPicker workspaceId="ws" value={{}} inDock onChange={onChange} />
    </TooltipProvider></QueryClientProvider>,
  ));
  await openPicker();
  const menu = document.querySelector('[data-dropdown-content]');
  expect(menu?.getAttribute("data-helpin-dock-overlay")).toBe("true");
  expect(menu?.classList.contains("z-[70]")).toBe(true);
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="personal"]')!.click());
  expect(onChange).toHaveBeenCalledWith({ ai_profile_id: "personal" });
});

it("disables a policy-rejected primary even when its fallback is allowed", async () => {
  const denied = { ...profiles[0], primary_policy: { allowed: false, message: "BYOK is disabled" },
    fallback: profiles[1].primary, fallback_policy: { allowed: true } };
  client.setQueryData(queryKeys.ai.profiles("ws"), [denied]);
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [denied], error: null });
  const onChange = vi.fn();
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIProfilePicker workspaceId="ws" value="personal" onChange={onChange} />
  </TooltipProvider></QueryClientProvider>));
  expect(document.body.textContent).toContain("BYOK is disabled");
  await openPicker();
  const item = document.querySelector<HTMLElement>('[cmdk-item][data-value="personal"]');
  expect(item?.getAttribute("aria-disabled")).toBe("true");
  await act(async () => item!.click());
  expect(onChange).not.toHaveBeenCalled();
});

it("discloses the inherited workspace route without selecting an override", async () => {
  const onChange = vi.fn();
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIProfilePicker workspaceId="ws" onChange={onChange} />
  </TooltipProvider></QueryClientProvider>));
  for (let i = 0; i < 20 && !document.body.textContent?.includes("Workspace default: workspace profile"); i++) {
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  }
  expect(document.body.textContent).toContain("workspace profile");
  expect(document.body.textContent).toContain("Default");
  expect(document.body.textContent).toContain("OpenAI · custom-model");
  expect(onChange).not.toHaveBeenCalled();
});

it("uses the agent default before the workspace default", async () => {
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIProfilePicker workspaceId="ws" defaultProfileId="workspace" onChange={() => {}} />
  </TooltipProvider></QueryClientProvider>));
  await openPicker();
  expect(document.body.textContent).toContain("workspace profile");
  expect(document.body.textContent).toContain("Default");
  expect(aiProfileService.settings).not.toHaveBeenCalled();
});

it("keeps the composer trigger to the profile name and orders default, workspace, then personal", async () => {
  const orderedProfiles: AIProfile[] = [
    { ...profiles[0], id: 'personal-a', name: 'Alpha personal' },
    { ...profiles[1], id: 'workspace-z', name: 'Zulu workspace' },
    { ...profiles[1], id: 'default-profile', name: 'Balanced' },
    { ...profiles[1], id: 'workspace-a', name: 'Alpha workspace' },
  ];
  client.setQueryData(queryKeys.ai.profiles('ws'), orderedProfiles);
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: orderedProfiles, error: null });
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIProfilePicker workspaceId="ws" compact defaultProfileId="default-profile" onChange={() => {}} />
  </TooltipProvider></QueryClientProvider>));
  expect(document.querySelector('label')).toBeNull();
  const trigger = document.querySelector('button[aria-haspopup="dialog"]');
  expect(trigger?.textContent).toBe('Balanced');
  expect(document.body.textContent).not.toContain('custom-model');
  await openPicker();
  expect([...document.querySelectorAll('[cmdk-item]')].map(el => el.getAttribute('data-value'))).toEqual(['default-profile', 'workspace-a', 'workspace-z', 'personal-a']);
  const row = document.querySelector('[cmdk-item][data-value="default-profile"]');
  expect(row?.textContent).toBe('BalancedDefaultWorkspace');
});

it("keeps an existing run locked even when its profile is no longer listed", async () => {
  useWorkspaceStore.setState({ currentWorkspace: { id: 'ws', slug: 'acme', name: 'Acme' } });
  client.setQueryData(queryKeys.ai.profiles('ws'), []);
  vi.mocked(aiProfileService.list).mockResolvedValue({data: [], error: null});
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIConnectionPicker workspaceId="ws" compact locked value={{ai_profile_id:'removed'}} onChange={() => {}} />
  </TooltipProvider></QueryClientProvider>));
  for (let i=0;i<20 && !document.querySelector('[data-slot="select-trigger"]');i++) {
    await act(async () => { await new Promise(resolve=>setTimeout(resolve,0)); });
  }
  const trigger=document.querySelector<HTMLButtonElement>('[data-slot="select-trigger"]');
  expect(trigger?.disabled).toBe(true);
  expect(trigger?.textContent).toBe('Saved model');
});

it.each([
  { surface: 'Ask Agent', inDock: true, sharedOnly: false },
  { surface: 'manual runs', inDock: false, sharedOnly: false },
  { surface: 'agent configuration', inDock: false, sharedOnly: true },
])('hides new choices in $surface while retaining the inherited default', async ({ inDock, sharedOnly }) => {
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [
    ...profiles,
    { ...profiles[1], id: 'hidden-workspace' },
  ].map(p => ({ ...p, hidden_from_ask_agent: true })), error: null });
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider><AIProfilePicker workspaceId="ws" inDock={inDock} sharedOnly={sharedOnly} compact onChange={() => {}} /></TooltipProvider></QueryClientProvider>));
  await openPicker();
  expect(document.querySelector('[cmdk-item][data-value="personal"]')).toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="hidden-workspace"]')).toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="workspace"]')).not.toBeNull();
});

it.each([true, false])('preserves an explicitly selected hidden model with sharedOnly=%s', async (sharedOnly) => {
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [
    ...profiles,
    { ...profiles[1], id: 'saved', name: 'Saved choice', hidden_from_ask_agent: true },
  ], error: null });
  const onChange = vi.fn();
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <AIProfilePicker workspaceId="ws" value="saved" sharedOnly={sharedOnly} onChange={onChange} />
  </TooltipProvider></QueryClientProvider>));
  await openPicker();
  expect(document.querySelector('[cmdk-item][data-value="saved"]')?.textContent).toContain('Saved choice');
  expect(onChange).not.toHaveBeenCalled();
});

it('filters delivery model choices but preserves its saved selection and agent defaults', async () => {
  const choices = [
    ...profiles.map(p => ({ ...p, hidden_from_ask_agent: true })),
    { ...profiles[1], id: 'visible', name: 'Visible choice' },
  ];
  client.setQueryData(queryKeys.ai.profiles('ws'), choices);
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: choices, error: null });
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>
    <EpicDeliveryProfileSelect workspaceId="ws" value="workspace" onChange={() => {}} />
  </TooltipProvider></QueryClientProvider>));
  await openPicker();
  expect(document.querySelector('[cmdk-item][data-value="personal"]')).toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="workspace"]')).not.toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="visible"]')).not.toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="__defaults__"]')).not.toBeNull();
});

it('uses the personal default only in Ask Agent and preserves explicit agent defaults', async () => {
  vi.mocked(aiProfileService.settings).mockResolvedValue({data:{default_profile_id:'workspace',personal_default_profile_id:'personal'},error:null});
  for (const [inDock, defaultProfileId, expected] of [[true, undefined, 'personal profile'], [false, undefined, 'workspace profile'], [true, 'workspace', 'workspace profile']] as const) {
    await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider><AIProfilePicker workspaceId="ws" inDock={inDock} compact defaultProfileId={defaultProfileId} onChange={() => {}} /></TooltipProvider></QueryClientProvider>));
    for (let i=0;i<8;i++) await act(async()=>{await new Promise(resolve=>setTimeout(resolve,0));});
    expect(document.querySelector('[data-slot="select-trigger"]')?.textContent).toBe(expected);
  }
});
