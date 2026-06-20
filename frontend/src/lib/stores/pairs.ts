// The pairs store holds saved sync/bisync pairs, named profiles, and per-remote
// governance ceilings, and drives their CRUD plus running a pair (§7.5–7.7).
import { writable } from "svelte/store";

import {
  ListPairs,
  SavePair,
  DeletePair,
  RunPair,
  ListProfiles,
  SaveProfile,
  DeleteProfile,
  ListCeilings,
  SetCeiling,
} from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const pairs = writable<app.PairDTO[]>([]);
export const profiles = writable<app.ProfileDTO[]>([]);
export const ceilings = writable<app.CeilingDTO[]>([]);
export const pairsError = writable<app.ErrorDTO | null>(null);

export async function refreshPairs(): Promise<void> {
  try {
    const res = await ListPairs();
    if (res.error) {
      pairsError.set(res.error);
    } else {
      pairs.set(res.pairs ?? []);
      pairsError.set(null);
    }
  } catch {
    // binding unavailable outside the webview
  }
}

export async function refreshProfiles(): Promise<void> {
  try {
    const res = await ListProfiles();
    if (!res.error) profiles.set(res.profiles ?? []);
  } catch {
    // binding unavailable outside the webview
  }
}

export async function refreshCeilings(): Promise<void> {
  try {
    const res = await ListCeilings();
    if (!res.error) ceilings.set(res.ceilings ?? []);
  } catch {
    // binding unavailable outside the webview
  }
}

export async function savePair(p: Partial<app.PairDTO>): Promise<app.ErrorDTO | null> {
  const err = await SavePair(p as app.PairDTO);
  await refreshPairs();
  return err ?? null;
}

export async function removePair(id: string): Promise<app.ErrorDTO | null> {
  const err = await DeletePair(id);
  await refreshPairs();
  return err ?? null;
}

// runPair runs a saved pair. A never-run pair runs as a dry-run regardless of
// acknowledged; a destructive live run needs acknowledged=true (§7.4).
export async function runPair(id: string, acknowledged: boolean): Promise<app.RunResultDTO> {
  const res = await RunPair(id, acknowledged);
  await refreshPairs(); // a first run flips hasRun
  return res;
}

export async function saveProfile(p: Partial<app.ProfileDTO>): Promise<app.ErrorDTO | null> {
  const err = await SaveProfile(p as app.ProfileDTO);
  await refreshProfiles();
  return err ?? null;
}

export async function removeProfile(id: string): Promise<app.ErrorDTO | null> {
  const err = await DeleteProfile(id);
  await Promise.all([refreshProfiles(), refreshPairs()]); // a deleted profile nulls pair refs
  return err ?? null;
}

export async function setCeiling(c: Partial<app.CeilingDTO>): Promise<app.ErrorDTO | null> {
  const err = await SetCeiling(c as app.CeilingDTO);
  await refreshCeilings();
  return err ?? null;
}
