// The mounts store holds the active mounts and drives mount/unmount (§7.11.6).
import { writable } from "svelte/store";

import { ListMounts, MountFs, UnmountFs } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export const mounts = writable<app.MountDTO[]>([]);
export const mountsError = writable<app.ErrorDTO | null>(null);

export async function refreshMounts(): Promise<void> {
  try {
    const res = await ListMounts();
    if (res.error) {
      mountsError.set(res.error);
    } else {
      mounts.set(res.mounts ?? []);
      mountsError.set(null);
    }
  } catch {
    // binding unavailable outside the webview
  }
}

export async function doMount(fs: string, mountPoint: string, mountType: string): Promise<app.ErrorDTO | null> {
  const err = await MountFs(fs, mountPoint, mountType);
  await refreshMounts();
  return err ?? null;
}

export async function doUnmount(mountPoint: string): Promise<app.ErrorDTO | null> {
  const err = await UnmountFs(mountPoint);
  await refreshMounts();
  return err ?? null;
}
