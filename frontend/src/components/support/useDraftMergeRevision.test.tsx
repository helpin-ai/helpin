// @vitest-environment jsdom
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, test, vi } from "vitest";
import { useDraftMergeRevision } from "./useDraftMergeRevision";

test("keeps an existing draft at its reviewed audience until explicitly confirmed", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  localStorage.clear();
  const container = document.createElement("div");
  const root = createRoot(container);
  let result!: ReturnType<typeof useDraftMergeRevision>;
  function Draft({ revision, dirty }: { revision: number; dirty: boolean }) {
    result = useDraftMergeRevision("ws", "conversation", revision, dirty);
    return null;
  }
  try {
    await act(async () => root.render(<Draft revision={0} dirty={false} />));
    await act(async () => root.render(<Draft revision={0} dirty />));
    await act(async () => root.render(<Draft revision={1} dirty />));
    expect(result.changed).toBe(true);
    expect(result.revision).toBe(0);
    await act(async () => result.acknowledge());
    expect(result.changed).toBe(false);
    expect(result.revision).toBe(1);
    await act(async () => root.render(<Draft revision={2} dirty={false} />));
    expect(result.changed).toBe(false);
    expect(result.revision).toBe(2);
  } finally {
    await act(async () => root.unmount());
    vi.unstubAllGlobals();
  }
});
