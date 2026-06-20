// runControls tracks the currently-started operation and drives StartRun /
// CancelRun (§7.11.4). The always-visible action flips between Start and Stop
// based on whether an operation is active.
import { writable } from "svelte/store";

import { StartRun, CancelRun } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";
import { builder } from "./builder";

export interface RunState {
  operationId: string | null;
  busy: boolean;
  error: app.ErrorDTO | null;
}

function createRunControls() {
  const { subscribe, set, update } = writable<RunState>({ operationId: null, busy: false, error: null });
  let state: RunState = { operationId: null, busy: false, error: null };
  subscribe((s) => (state = s));

  async function start(acknowledged: boolean): Promise<void> {
    const sel = builder.current();
    update((s) => ({ ...s, busy: true, error: null }));
    try {
      const res = await StartRun(
        {
          kind: sel.kind,
          single: sel.single,
          multi: sel.multi,
          ceilings: sel.ceilings,
          src: sel.src,
          dst: sel.dst,
        } as app.PreviewRequest,
        acknowledged,
      );
      if (res.error) {
        set({ operationId: null, busy: false, error: res.error });
      } else {
        set({ operationId: res.operationId, busy: false, error: null });
      }
    } catch {
      update((s) => ({ ...s, busy: false }));
    }
  }

  async function stop(): Promise<void> {
    if (!state.operationId) return;
    const id = state.operationId;
    update((s) => ({ ...s, busy: true }));
    try {
      await CancelRun(id);
    } finally {
      set({ operationId: null, busy: false, error: null });
    }
  }

  // clearIfDone resets the active run once the daemon reports no active jobs,
  // so the control returns to Start after a natural completion.
  function clearIfDone(activeJobs: number): void {
    if (state.operationId && activeJobs === 0) {
      set({ operationId: null, busy: false, error: null });
    }
  }

  return { subscribe, start, stop, clearIfDone };
}

export const runControls = createRunControls();
