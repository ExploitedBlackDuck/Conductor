// The run store holds live transfer state (runtime state, §7.11.10). It seeds
// from StatsSnapshot and then updates from the typed EventStatsUpdate event,
// validated at the boundary before use (§2.8).
import { writable } from "svelte/store";

import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
import { StatsSnapshot } from "../../../wailsjs/go/app/App";
import { app } from "../../../wailsjs/go/models";

const EVENT_STATS_UPDATE = "stats:update";

// isStatsEvent validates an untyped event payload before it is trusted.
function isStatsEvent(v: unknown): v is app.StatsEventDTO {
  if (typeof v !== "object" || v === null) return false;
  const o = v as Record<string, unknown>;
  return typeof o.bytes === "number" && typeof o.speed === "number" && Array.isArray(o.transferring);
}

function createRunStore() {
  const { subscribe, set } = writable<app.StatsEventDTO | null>(null);
  let bound = false;

  async function start(): Promise<void> {
    if (bound) return;
    bound = true;
    try {
      set(await StatsSnapshot());
    } catch {
      // binding unavailable outside the webview
    }
    EventsOn(EVENT_STATS_UPDATE, (payload: unknown) => {
      if (isStatsEvent(payload)) {
        set(app.StatsEventDTO.createFrom(payload));
      }
    });
  }

  function stop(): void {
    if (!bound) return;
    EventsOff(EVENT_STATS_UPDATE);
    bound = false;
  }

  return { subscribe, start, stop };
}

export const run = createRunStore();
