<script lang="ts">
  import { builder, type OperationKind } from "../stores/builder";

  const selectionStore = builder.selection;
  $: sel = $selectionStore;

  const kinds: OperationKind[] = ["copy", "sync", "move", "bisync", "delete", "purge"];

  // Endpoints are entered as remote:path. A leading "remote:" is split out so
  // the resolved endpoint is shown explicitly (§7.4 — no silent interpolation).
  function parseEndpoint(text: string): { remote: string; path: string } {
    const idx = text.indexOf(":");
    if (idx === -1) return { remote: "", path: text };
    return { remote: text.slice(0, idx), path: text.slice(idx + 1) };
  }
  function onKind(e: Event) {
    builder.setKind((e.target as HTMLSelectElement).value as OperationKind);
  }
  function onEndpoint(which: "src" | "dst", e: Event) {
    builder.setEndpoint(which, parseEndpoint((e.target as HTMLInputElement).value));
  }
  function display(ep: { remote: string; path: string }): string {
    return ep.remote ? `${ep.remote}:${ep.path}` : ep.path;
  }

  // A second endpoint is not meaningful for delete/purge.
  $: needsDest = !["delete", "purge"].includes(sel.kind);
</script>

<div class="picker">
  <label class="field">
    <span>Operation</span>
    <select value={sel.kind} on:change={onKind}>
      {#each kinds as k (k)}
        <option value={k}>{k}</option>
      {/each}
    </select>
  </label>

  <label class="field">
    <span>Source</span>
    <input
      type="text"
      placeholder="remote:path or /local/path"
      value={display(sel.src)}
      on:input={(e) => onEndpoint("src", e)}
    />
  </label>

  {#if needsDest}
    <label class="field">
      <span>Destination</span>
      <input
        type="text"
        placeholder="remote:path or /local/path"
        value={display(sel.dst)}
        on:input={(e) => onEndpoint("dst", e)}
      />
    </label>
  {/if}
</div>

<style>
  .picker {
    display: grid;
    gap: var(--space-3);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .field span {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  select,
  input {
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.45rem 0.55rem;
    font-size: 0.9rem;
    font-family: inherit;
  }
</style>
