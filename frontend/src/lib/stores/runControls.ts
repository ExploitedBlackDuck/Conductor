// runControls tracks the currently-started operation and drives StartRun /
// CancelRun (§7.11.4). The always-visible action flips between Start and Stop
// based on whether an operation is active.
import { writable } from "svelte/store";

import { StartRun, PreviewRun, CancelRun } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";
import { builder } from "./builder";

export interface RunState {
  operationId: string | null;
  busy: boolean;
  error: app.ErrorDTO | null;
  // changeSet is the dry-run preview the operator must see before a destructive
  // confirm unlocks (ADR-0015); null until previewed.
  changeSet: app.ChangeSetDTO | null;
}

const initial: RunState = { operationId: null, busy: false, error: null, changeSet: null };

function selectionRequest(): app.PreviewRequest {
  const sel = builder.current();
  return {
    kind: sel.kind,
    single: sel.single,
    multi: sel.multi,
    ceilings: sel.ceilings,
    src: sel.src,
    dst: sel.dst,
  } as app.PreviewRequest;
}

function createRunControls() {
  const { subscribe, set, update } = writable<RunState>({ ...initial });
  let state: RunState = { ...initial };
  subscribe((s) => (state = s));

  // previewChanges runs the dry-run and stores the change set the operator must
  // see before confirming a destructive run (ADR-0015).
  async function previewChanges(): Promise<void> {
    update((s) => ({ ...s, busy: true, error: null }));
    try {
      const res = await PreviewRun(selectionRequest());
      if (res.error) {
        update((s) => ({ ...s, busy: false, error: res.error ?? null, changeSet: null }));
      } else {
        update((s) => ({ ...s, busy: false, changeSet: res.changeSet }));
      }
    } catch {
      update((s) => ({ ...s, busy: false }));
    }
  }

  async function start(acknowledged: boolean): Promise<void> {
    update((s) => ({ ...s, busy: true, error: null }));
    try {
      const res = await StartRun(selectionRequest(), acknowledged);
      if (res.error) {
        update((s) => ({ ...s, operationId: null, busy: false, error: res.error ?? null }));
      } else {
        set({ operationId: res.operationId, busy: false, error: null, changeSet: null });
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
      set({ ...initial });
    }
  }

  // clearIfDone resets the active run once the daemon reports no active jobs,
  // so the control returns to Start after a natural completion.
  function clearIfDone(activeJobs: number): void {
    if (state.operationId && activeJobs === 0) {
      set({ ...initial });
    }
  }

  // clearPreview drops a stale change set (e.g. when the selection changes).
  function clearPreview(): void {
    if (state.changeSet) update((s) => ({ ...s, changeSet: null }));
  }

  return { subscribe, start, stop, previewChanges, clearPreview, clearIfDone };
}

export const runControls = createRunControls();
