import { describe, expect, it } from "vitest";
import { mergeSourceIds, canMergeSelection } from "./supportMerging";

describe("conversation merge selection", () => {
  it("keeps every selected original except the explicitly chosen destination", () => {
    expect(mergeSourceIds(["a", "b", "c"], "b")).toEqual(["a", "c"]);
    expect(mergeSourceIds(["a", "b"], "c")).toEqual(["a", "b"]);
  });
  it("rejects an empty, oversized, or already merged selection", () => {
    expect(canMergeSelection([])).toBe(false);
    expect(
      canMergeSelection([{ id: "a" }, { id: "b", merged_into_id: "c" }]),
    ).toBe(false);
    expect(
      canMergeSelection(Array.from({ length: 51 }, (_, i) => ({ id: `${i}` }))),
    ).toBe(false);
    expect(canMergeSelection([{ id: "a" }, { id: "b" }])).toBe(true);
  });
});
