// The builder store holds the operator's current operation selection (view
// state) and the live resolved-operation preview (runtime state), kept distinct
// per §7.11.10. Mutating the selection debounces a PreviewOperation call so the
// impact panel and command preview stay in sync without thrashing.
import { writable, get } from "svelte/store";

import { PreviewOperation } from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

export type OperationKind = "copy" | "sync" | "move" | "bisync" | "delete" | "purge";

export interface Endpoint {
  remote: string;
  path: string;
}

export interface SelectionState {
  kind: OperationKind;
  single: Record<string, string>;
  multi: Record<string, string[]>;
  src: Endpoint;
  dst: Endpoint;
  ceilings: { transfers: number; checkers: number; bwlimit: string; tpslimit: number };
}

function initialSelection(): SelectionState {
  return {
    kind: "copy",
    single: {},
    multi: {},
    src: { remote: "", path: "" },
    dst: { remote: "", path: "" },
    // Conservative defaults (ADR-0013); 0/"" mean "no ceiling" to the backend.
    ceilings: { transfers: 0, checkers: 0, bwlimit: "", tpslimit: 0 },
  };
}

function createBuilder() {
  const selection = writable<SelectionState>(initialSelection());
  const preview = writable<app.PreviewDTO | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function refresh(sel: SelectionState): Promise<void> {
    try {
      preview.set(
        await PreviewOperation({
          kind: sel.kind,
          single: sel.single,
          multi: sel.multi,
          ceilings: sel.ceilings,
          src: sel.src,
          dst: sel.dst,
        } as app.PreviewRequest),
      );
    } catch {
      // binding unavailable outside the webview
    }
  }

  selection.subscribe((sel) => {
    if (timer !== undefined) clearTimeout(timer);
    timer = setTimeout(() => void refresh(sel), 120);
  });

  function update(mut: (s: SelectionState) => void): void {
    selection.update((s) => {
      mut(s);
      return s;
    });
  }

  return {
    selection,
    preview,
    setKind: (kind: OperationKind) => update((s) => (s.kind = kind)),
    setSingle: (flag: string, value: string) =>
      update((s) => {
        if (value === "" || value === "false") {
          delete s.single[flag];
        } else {
          s.single[flag] = value;
        }
      }),
    setMulti: (flag: string, values: string[]) =>
      update((s) => {
        const nonEmpty = values.filter((v) => v.trim() !== "");
        if (nonEmpty.length === 0) {
          delete s.multi[flag];
        } else {
          s.multi[flag] = nonEmpty;
        }
      }),
    setEndpoint: (which: "src" | "dst", ep: Endpoint) => update((s) => (s[which] = ep)),
    reset: () => selection.set(initialSelection()),
    current: () => get(selection),
  };
}

export const builder = createBuilder();
