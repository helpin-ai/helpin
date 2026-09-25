// Compose health checks do not cover nginx's DNS refresh after API replacement.
export async function waitForAPI(base, mailConfigured, timeout = 30000) {
  const deadline = Date.now() + timeout;
  let status = 'unreachable';
  do {
    try {
      const response = await fetch(`${base}/api/auth/config`, { signal: AbortSignal.timeout(2000) });
      status = response.status;
      if (response.ok) {
        const config = await response.json();
        if (config.public_widget_url === base && config.app_email_configured === mailConfigured) return config;
        status = 'previous configuration';
      }
    } catch { status = 'unreachable'; }
    if (Date.now() >= deadline) break;
    await new Promise(resolve => setTimeout(resolve, Math.min(250, deadline - Date.now())));
  } while (Date.now() <= deadline);
  throw new Error(`Public API did not reach the expected configuration: ${status}`);
}
