// The catalog store holds the option catalog, loaded once from the backend.
import { writable } from "svelte/store";

import { GetCatalog } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const catalog = writable<app.CatalogDTO | null>(null);

export async function loadCatalog(): Promise<void> {
  try {
    catalog.set(await GetCatalog());
  } catch {
    // Outside the Wails webview the binding is absent; leave null.
  }
}
