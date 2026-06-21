// The verify store holds past integrity checks and drives running a new one
// (§7.12). A verification mutates nothing; a mismatch is a result, not an error.
import { writable } from "svelte/store";

import { RunVerify, ListVerifications } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const verifications = writable<app.VerificationDTO[]>([]);
export const verifyError = writable<app.ErrorDTO | null>(null);

export async function refreshVerifications(): Promise<void> {
  try {
    const res = await ListVerifications();
    if (res.error) {
      verifyError.set(res.error);
    } else {
      verifications.set(res.verifications ?? []);
      verifyError.set(null);
    }
  } catch {
    // binding unavailable outside the webview
  }
}

export async function runVerify(
  kind: "check" | "cryptcheck",
  src: app.EndpointDTO,
  dst: app.EndpointDTO,
  oneway: boolean,
): Promise<app.VerifyResultDTO> {
  const res = await RunVerify(kind, src, dst, oneway);
  await refreshVerifications();
  return res;
}
