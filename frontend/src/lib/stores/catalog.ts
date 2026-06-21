// The catalog store holds the option catalog, loaded once from the backend, and
// surfaces an explicit error state so the option builder is never left blank
// (§7.11.9).
import { writable } from "svelte/store";

import { GetCatalog } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const catalog = writable<app.CatalogDTO | null>(null);
export const catalogError = writable<string | null>(null);
export const catalogLoading = writable<boolean>(false);

export async function loadCatalog(): Promise<void> {
  catalogLoading.set(true);
  catalogError.set(null);
  try {
    catalog.set(await GetCatalog());
  } catch {
    // The binding threw (or is absent outside the webview). Don't sit on a
    // perpetual "loading" — record a degraded state the panel can render.
    catalogError.set("The option catalog could not be loaded.");
  } finally {
    catalogLoading.set(false);
  }
}
