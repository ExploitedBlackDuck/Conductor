<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import RiskBadge from "./RiskBadge.svelte";

  export let preview: app.PreviewDTO | null;
</script>

<div class="preview">
  {#if preview === null}
    <p class="muted">Choose a source and destination to begin.</p>
  {:else}
    <div class="resolved">
      <div><span class="k">Operation</span><span class="v">{preview.kind}</span></div>
      <div><span class="k">Source</span><span class="v mono">{preview.resolvedSrc || "—"}</span></div>
      <div><span class="k">Destination</span><span class="v mono">{preview.resolvedDst || "—"}</span></div>
      <div><span class="k">Risk</span><span class="v"><RiskBadge risk={preview.riskLevel} /></span></div>
    </div>

    {#if preview.error}
      <p class="err" role="alert"><strong>{preview.error.code}</strong> — {preview.error.message}</p>
    {:else}
      <div class="cmd">
        <span class="cmd-label">Effective command</span>
        <code>{preview.command}</code>
      </div>
    {/if}
  {/if}
</div>

<style>
  .preview {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .resolved {
    display: grid;
    gap: var(--space-2);
  }
  .resolved div {
    display: flex;
    gap: var(--space-3);
    align-items: center;
  }
  .k {
    width: 6.5rem;
    color: var(--color-text-muted);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .v.mono,
  .cmd code {
    font-family: var(--font-mono);
    font-size: 0.82rem;
  }
  .cmd {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .cmd-label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  .cmd code {
    display: block;
    padding: var(--space-3);
    background: #0a0d11;
    border: 1px solid var(--color-border);
    border-radius: 6px;
    line-height: 1.6;
    word-break: break-all;
  }
  .err {
    margin: 0;
    padding: var(--space-3);
    border: 1px solid #5c2b29;
    border-radius: 6px;
    background: #2d1513;
    color: #ffb3ab;
    font-size: 0.85rem;
  }
  .muted {
    color: var(--color-text-muted);
    margin: 0;
  }
</style>
