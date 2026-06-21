// Native completion notifications via the webview's Notification API (§7.13),
// routed through the frontend so there is no platform-specific Go dependency.
// Notifications are off for short, noisy operations and shown only for long
// ones (see LONG_OPERATION_MS).

// LONG_OPERATION_MS is the threshold above which a completed operation is worth
// a notification — a quick copy shouldn't interrupt the operator.
export const LONG_OPERATION_MS = 10_000;

function available(): boolean {
  return typeof Notification !== "undefined";
}

// ensurePermission requests notification permission once, lazily (e.g. when the
// first operation starts) rather than on app load.
export async function ensurePermission(): Promise<void> {
  if (!available() || Notification.permission !== "default") return;
  try {
    await Notification.requestPermission();
  } catch {
    // permission API unavailable; notifications simply stay off
  }
}

// notify shows a native notification if permission was granted; otherwise it is
// a no-op, so a denied prompt never blocks anything.
export function notify(title: string, body: string): void {
  if (!available() || Notification.permission !== "granted") return;
  try {
    new Notification(title, { body });
  } catch {
    // some webviews disallow construction; ignore
  }
}
