import { describe, it, expect } from "vitest";
import { emailStarters, buildStarterSteps } from "../emailStarters";
describe("scenario starter content", () => {
  it("requires real context and never silently uses example business claims", () => {
    expect(() => buildStarterSteps(emailStarters[0], {})).toThrow(/Complete/);
  });
  it("creates independent reviewed steps with safe HTML and supported recipient variables", () => {
    for (const starter of emailStarters) {
      const fields = Object.fromEntries(
        starter.fields.map((field) => [field.key, "Acme <script> & team"]),
      );
      const steps = buildStarterSteps(starter, fields);
      expect(steps).toHaveLength(3);
      expect(steps[0].delay_days).toBe(0);
      expect(steps.every((step) => step.mode === "review")).toBe(true);
      expect(
        steps.every(
          (step) =>
            !step.subject?.includes("[[") && !step.body_html?.includes("[["),
        ),
      ).toBe(true);
      expect(steps.map((step) => step.body_html).join("")).not.toContain(
        "<script>",
      );
      expect(steps.map((step) => step.body_html).join("")).toContain(
        "&lt;script&gt;",
      );
      expect(steps[0].body_html).toContain("{{first_name|there}}");
      steps[0].subject = "Edited";
      expect(buildStarterSteps(starter, fields)[0].subject).not.toBe("Edited");
    }
  });
  it("rejects accidental merge variables and header newlines in setup values", () => {
    const starter = emailStarters[0];
    const values = Object.fromEntries(
      starter.fields.map((f) => [f.key, "Useful context"]),
    );
    expect(() =>
      buildStarterSteps(starter, {
        ...values,
        [starter.fields[0].key]: "{{unknown}}",
      }),
    ).toThrow();
    expect(() =>
      buildStarterSteps(starter, {
        ...values,
        [starter.fields[0].key]: "Hello\nBcc: someone",
      }),
    ).toThrow();
  });
});
