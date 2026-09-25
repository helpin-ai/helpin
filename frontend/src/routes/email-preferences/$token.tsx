import { createFileRoute } from "@tanstack/react-router";
import { API_BASE } from "@/lib/api";
export const Route = createFileRoute("/email-preferences/$token")({
  component: EmailPreferences,
});
function EmailPreferences() {
  const { token } = Route.useParams();
  const valid = /^[a-f0-9-]{72}$/.test(token);
  return (
    <main className="mx-auto max-w-lg px-6 py-24 text-foreground">
      <h1 className="text-xl font-semibold">
        {valid ? "Stop follow-up emails?" : "Link unavailable"}
      </h1>
      {valid && (
        <>
          <p className="mt-3 text-sm text-muted-foreground">
            You will no longer receive automated email sequences from this
            workspace.
          </p>
          <form
            className="mt-6"
            method="post"
            action={`${API_BASE}/crm/outreach/unsubscribe/${encodeURIComponent(token)}`}
          >
            <button
              className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground"
              type="submit"
            >
              Unsubscribe
            </button>
          </form>
        </>
      )}
    </main>
  );
}
