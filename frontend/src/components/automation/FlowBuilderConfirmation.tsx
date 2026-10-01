import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { ReactNode } from "react";
export function FlowBuilderConfirmation({
  editing,
  valid,
  preview,
  onResolve,
}: {
  editing: boolean;
  valid: boolean;
  preview: ReactNode;
  onResolve: (
    decision: "approve" | "request_changes",
    message?: string,
  ) => Promise<{
    error: string | null;
  }>;
}) {
  const [changing, setChanging] = useState(false),
    [note, setNote] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const resolve = async (decision: "approve" | "request_changes") => {
    setBusy(true);
    setError("");
    try {
      const result = await onResolve(decision, note.trim() || undefined);
      if (result.error) setError(result.error);
    } catch {
      setError("Could not send your response. Try again.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <div>
      <h3 className="text-base font-semibold">
        {editing ? "Save these changes?" : "Create this flow?"}
      </h3>
      {preview}
      {!valid && (
        <p className="mb-3 text-sm text-destructive">
          This preview has changed. Ask for an updated review.
        </p>
      )}
      {changing && (
        <Textarea
          autoFocus
          aria-label="Requested changes"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder="What would you like to change?"
          className="mb-3"
        />
      )}
      {error && (
        <p role="alert" className="mb-3 text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex justify-end gap-2">
        {changing ? (
          <>
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => setChanging(false)}
            >
              Back
            </Button>
            <Button
              disabled={busy || !note.trim()}
              onClick={() => void resolve("request_changes")}
            >
              {busy ? "Sending…" : "Send changes"}
            </Button>
          </>
        ) : (
          <>
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => setChanging(true)}
            >
              Make changes
            </Button>
            <Button
              disabled={busy || !valid}
              onClick={() => void resolve("approve")}
            >
              {busy
                ? "Confirming…"
                : editing
                  ? "Yes, save changes"
                  : "Yes, create flow"}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
