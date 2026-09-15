import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { usePollAIConnection } from "@/hooks/queries/useAIConnections";
import type { AIConnectionLogin } from "@/lib/services/aiConnectionService";
import {
  Alert01Icon,
  CheckmarkCircle02Icon,
  Copy01Icon,
  Loading01Icon,
} from "@/lib/icons";
import { Button } from "@/components/ui/button";

export type DeviceLoginPhase = "waiting" | "connected" | "failed" | "expired";

function remainingLabel(expiresAt: string | undefined, now: number): string | null {
  if (!expiresAt) return null;
  const end = new Date(expiresAt).getTime();
  if (!Number.isFinite(end)) return null;
  const seconds = Math.max(0, Math.round((end - now) / 1000));
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
}

function isExpired(expiresAt: string | undefined, now: number): boolean {
  if (!expiresAt) return false;
  const end = new Date(expiresAt).getTime();
  return Number.isFinite(end) && end <= now;
}

/**
 * Polls one pending ChatGPT device login. Polling stops on success, failure,
 * expiry, and unmount, so a cancelled dialog never keeps a timer alive.
 */
export function useChatGPTDeviceLogin({
  workspaceId,
  login,
  onLogin,
  onConnected,
}: {
  workspaceId: string;
  login?: AIConnectionLogin;
  onLogin: (login: AIConnectionLogin) => void;
  onConnected?: (login: AIConnectionLogin) => void;
}) {
  const poll = usePollAIConnection(workspaceId);
  const [failure, setFailure] = useState("");
  const [now, setNow] = useState(() => Date.now());
  const connectionId = login?.connection.id;
  const status = login?.connection.status;
  const expiresAt = login?.expires_at;
  const intervalSeconds = Math.max(5, login?.interval_seconds || 5);
  const expired = isExpired(expiresAt, now);

  const phase: DeviceLoginPhase = status === "connected"
    ? "connected"
    : failure
      ? "failed"
      : expired
        ? "expired"
        : "waiting";

  const onLoginRef = useRef(onLogin);
  onLoginRef.current = onLogin;
  const onConnectedRef = useRef(onConnected);
  onConnectedRef.current = onConnected;
  const pollRef = useRef(poll);
  pollRef.current = poll;

  const checkNow = useCallback(async () => {
    if (!connectionId) return;
    setFailure("");
    try {
      const next = await pollRef.current.mutateAsync(connectionId);
      onLoginRef.current(next);
      if (next.connection.status === "connected") onConnectedRef.current?.(next);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "Unable to check the login.");
    }
  }, [connectionId]);

  // Tick only while a code is outstanding so the countdown and expiry are live.
  useEffect(() => {
    if (phase !== "waiting" || !expiresAt) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [phase, expiresAt]);

  useEffect(() => {
    if (phase !== "waiting" || !connectionId) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      if (!cancelled) void checkNow();
    }, intervalSeconds * 1000);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [phase, connectionId, intervalSeconds, checkNow, login]);

  return {
    phase,
    failure,
    checkNow,
    countdown: remainingLabel(expiresAt, now),
    isChecking: poll.isPending,
  };
}

export function ChatGPTDeviceLogin({
  login,
  phase,
  failure,
  countdown,
  isChecking,
  onCheckNow,
  onStartAgain,
}: {
  login: AIConnectionLogin;
  phase: DeviceLoginPhase;
  failure: string;
  countdown: string | null;
  isChecking: boolean;
  onCheckNow: () => void;
  onStartAgain: () => void;
}) {
  if (phase === "connected")
    return (
      <div role="status" className="flex items-start gap-2 text-sm">
        <CheckmarkCircle02Icon className="mt-0.5 h-4 w-4 text-emerald-600" />
        <p>ChatGPT connected. New runs use this connection when you select its profile.</p>
      </div>
    );

  if (phase === "expired" || phase === "failed")
    return (
      <div role="status" className="space-y-3 text-sm">
        <div className="flex items-start gap-2">
          <Alert01Icon className="mt-0.5 h-4 w-4 text-amber-600" />
          <p>
            {phase === "expired"
              ? "The code expired before it was approved."
              : failure || "The login could not be confirmed."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {phase === "failed" && (
            <Button type="button" size="sm" variant="outline" onClick={onCheckNow} disabled={isChecking}>
              Check again
            </Button>
          )}
          <Button type="button" size="sm" onClick={onStartAgain}>
            Start again
          </Button>
        </div>
      </div>
    );

  return (
    <div role="status" className="space-y-3">
      <p className="text-sm">Enter this code to connect your ChatGPT account:</p>
      <div className="flex items-center gap-2">
        <p className="select-all font-mono text-xl tracking-[0.2em]">{login.user_code}</p>
        <button
          type="button"
          title="Copy code"
          className="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
          onClick={() => {
            void navigator.clipboard
              ?.writeText(login.user_code ?? "")
              .then(() => toast.success("Code copied"))
              .catch(() => toast.error("Could not copy the code"));
          }}
        >
          <Copy01Icon className="h-3.5 w-3.5" />
          <span className="sr-only">Copy code</span>
        </button>
      </div>
      {login.verification_url && (
        <Button asChild size="sm" variant="outline">
          <a href={login.verification_url} target="_blank" rel="noopener noreferrer">
            Open ChatGPT device login
          </a>
        </Button>
      )}
      <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Waiting for approval
        {countdown ? ` · Expires in ${countdown}` : " · The code expires in a few minutes"}
      </p>
    </div>
  );
}
