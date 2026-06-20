// The status store holds the live, polled daemon/transfer status (§7.11.10:
// runtime state in a defined store). In P2 it polls the Status binding on an
// interval; P4 replaces polling with the typed event stream.
import { writable } from "svelte/store";

import { Status } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export type StatusData = app.StatusDTO;

function createStatusStore() {
  const { subscribe, set } = writable<StatusData | null>(null);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function poll(): Promise<void> {
    try {
      set(await Status());
    } catch {
      // Outside the Wails webview window.go is absent; leave the last value.
    }
  }

  function start(intervalMs = 1000): void {
    void poll();
    timer = setInterval(() => void poll(), intervalMs);
  }

  function stop(): void {
    if (timer !== undefined) {
      clearInterval(timer);
      timer = undefined;
    }
  }

  return { subscribe, start, stop, poll };
}

export const status = createStatusStore();
