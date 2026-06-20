// The history store holds the operation history, per-operation detail, and the
// audit-chain view, and drives queries, export, and clear (§7.11.7–7.11.8).
import { writable } from "svelte/store";

import {
  RecentHistory,
  HistoryByRemote,
  DestructiveHistory,
  OperationDetail,
  AuditView,
  ExportHistory,
  ClearHistory,
} from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const operations = writable<app.OperationDTO[]>([]);
export const historyError = writable<app.ErrorDTO | null>(null);
export const audit = writable<app.AuditViewDTO | null>(null);

export type Filter = { kind: "recent" } | { kind: "destructive" } | { kind: "remote"; remote: string };

export async function loadHistory(filter: Filter): Promise<void> {
  try {
    let res: app.OperationsResultDTO;
    if (filter.kind === "destructive") {
      res = await DestructiveHistory();
    } else if (filter.kind === "remote") {
      res = await HistoryByRemote(filter.remote);
    } else {
      res = await RecentHistory(500);
    }
    if (res.error) {
      historyError.set(res.error);
    } else {
      operations.set(res.operations ?? []);
      historyError.set(null);
    }
  } catch {
    // binding unavailable outside the webview
  }
}

export async function detail(id: string): Promise<app.OperationDetailDTO | null> {
  try {
    return await OperationDetail(id);
  } catch {
    return null;
  }
}

export async function loadAudit(): Promise<void> {
  try {
    audit.set(await AuditView());
  } catch {
    // binding unavailable outside the webview
  }
}

// exportHistory fetches an export and triggers a browser download of the file.
export async function exportHistory(format: "json" | "csv", remote: string): Promise<app.ErrorDTO | null> {
  const res = await ExportHistory(format, remote);
  if (res.error) return res.error;
  downloadBase64(res.filename, res.base64, format === "csv" ? "text/csv" : "application/json");
  return null;
}

export async function clearHistory(): Promise<app.ErrorDTO | null> {
  const err = await ClearHistory();
  await loadHistory({ kind: "recent" });
  return err ?? null;
}

function downloadBase64(filename: string, b64: string, mime: string): void {
  const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
  const url = URL.createObjectURL(new Blob([bytes], { type: mime }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
