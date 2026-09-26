// @vitest-environment jsdom
import { act } from "react";
import type { WorkspaceBillingSummary } from "@/lib/types";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import {
  BillingSettingsPage,
  canRemoveHelpinBranding,
  TrialBanner,
  UpgradeRequiredDialog,
  useWorkspaceBilling,
  WorkspaceBillingGate,
  workspaceBillingBadge,
} from "../index";
import { getUpgradeRequiredReason, isUpgradeRequiredError } from "../errors";

it("does not fetch billing or gate content when historical billing metadata is locked", async () => {
  const fetch = vi
    .spyOn(globalThis, "fetch")
    .mockRejectedValue(new Error("unexpected request"));
  const container = document.createElement("div");
  const root = createRoot(container);
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  function Workspace() {
    const billing = useWorkspaceBilling("workspace");
    expect(billing).toEqual({ data: undefined, isLoading: false });
    return (
      <WorkspaceBillingGate
        slug="workspace"
        billing={{ locked: true } as WorkspaceBillingSummary}
      >
        <p>Workspace content</p>
        <TrialBanner />
        <BillingSettingsPage />
        <UpgradeRequiredDialog
          open
          reason="workspace is locked"
          onOpenChange={() => {}}
        />
      </WorkspaceBillingGate>
    );
  }
  try {
    await act(async () => {
      root.render(<Workspace />);
    });
    expect(container.textContent).toBe("Workspace content");
    expect(fetch).not.toHaveBeenCalled();
    expect(
      workspaceBillingBadge({ locked: true } as WorkspaceBillingSummary),
    ).toBeNull();
    expect(canRemoveHelpinBranding({ plan: "starter", locked: true })).toBe(
      true,
    );
    expect(
      getUpgradeRequiredReason(new Error("custom AI agents require Growth")),
    ).toBeNull();
    expect(isUpgradeRequiredError(new Error("workspace is locked"))).toBe(
      false,
    );
  } finally {
    await act(async () => {
      root.unmount();
    });
    fetch.mockRestore();
  }
});
