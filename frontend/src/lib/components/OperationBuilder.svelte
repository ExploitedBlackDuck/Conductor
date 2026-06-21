<script lang="ts">
  import { onMount } from "svelte";
  import { catalog, loadCatalog } from "../stores/catalog";
  import { builder } from "../stores/builder";
  import SourceDestPicker from "./SourceDestPicker.svelte";
  import OptionBuilder from "./OptionBuilder.svelte";
  import CommandPreview from "./CommandPreview.svelte";
  import ImpactPanel from "./ImpactPanel.svelte";
  import RunControls from "./RunControls.svelte";

  const previewStore = builder.preview;
  const selectionStore = builder.selection;
  onMount(() => void loadCatalog());

  // A copy/move between paths on the same remote can run server-side (§7.11.3),
  // so data is not proxied through the operator's link.
  $: sel = $selectionStore;
  $: serverSideEligible = !!sel.src.remote && sel.src.remote === sel.dst.remote;
</script>

<div class="layout">
  <div class="left">
    <section class="card">
      <h2>Operation</h2>
      <SourceDestPicker />
    </section>

    <section class="card">
      <h2>Options</h2>
      {#if $catalog}
        <p class="version">rclone {$catalog.rcloneVersion}</p>
        <OptionBuilder categories={$catalog.categories} />
      {:else}
        <p class="muted">Loading option catalog…</p>
      {/if}
    </section>
  </div>

  <aside class="right">
    <section class="card sticky">
      <h2>Resolved operation</h2>
      <CommandPreview preview={$previewStore} />
      {#if serverSideEligible}
        <p class="note ok" title="Source and destination are on the same remote">
          ↔ Server-side eligible — rclone can copy/move on the remote without using your link.
        </p>
      {/if}
      <p class="note muted">Bandwidth and concurrency caps apply per operation, not shared across runs.</p>
      <div class="run">
        <RunControls preview={$previewStore} />
      </div>
    </section>
    <section class="card">
      <h2>Impact</h2>
      <ImpactPanel
        impacts={$previewStore?.impacts ?? []}
        clamps={$previewStore?.clamps ?? []}
      />
    </section>
  </aside>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 22rem);
    gap: var(--space-4);
    align-items: start;
  }
  .left,
  .right {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
  }
  .card {
    border: 1px solid var(--color-border);
    border-radius: 10px;
    background: var(--color-surface);
    padding: var(--space-4);
  }
  .card h2 {
    margin: 0 0 var(--space-3);
    font-size: 0.95rem;
  }
  .sticky {
    position: sticky;
    top: var(--space-4);
  }
  .run {
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }
  .version {
    margin: 0 0 var(--space-3);
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }
  .note {
    margin: var(--space-3) 0 0;
    font-size: 0.78rem;
  }
  .note.ok {
    color: #7ee787;
  }
  .note.muted {
    color: var(--color-text-muted);
  }
  .muted {
    color: var(--color-text-muted);
  }

  /* Below the minimum width, the preview pane drops below the builder rather
     than crushing it (§7.11.1). */
  @media (max-width: 880px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
</style>
