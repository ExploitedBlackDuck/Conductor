<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";

  export let impacts: app.ImpactDTO[];
  export let clamps: app.ClampDTO[] = [];

  const order: Record<string, number> = { block: 0, require_ack: 1, warn: 2 };
  $: sorted = [...impacts].sort((a, b) => (order[a.level] ?? 9) - (order[b.level] ?? 9));

  const label: Record<string, string> = {
    block: "Blocked",
    require_ack: "Acknowledgement required",
    warn: "Warning",
  };
</script>

<div class="impact">
  {#if sorted.length === 0 && clamps.length === 0}
    <p class="ok"><span aria-hidden="true">✓</span> No warnings for this selection.</p>
  {/if}

  {#each sorted as im (im.level + im.title + im.flag)}
    <div class="finding {im.level}">
      <div class="line">
        <span class="tag">{label[im.level] ?? im.level}</span>
        {#if im.flag}<code>{im.flag}</code>{/if}
        <strong>{im.title}</strong>
      </div>
      <p>{im.detail}</p>
    </div>
  {/each}

  {#each clamps as cl (cl.flag)}
    <div class="finding clamp">
      <div class="line">
        <span class="tag">Clamped</span>
        <code>{cl.flag}</code>
        <strong>{cl.requested} → {cl.applied}</strong>
      </div>
      <p>{cl.reason}</p>
    </div>
  {/each}
</div>

<style>
  .impact {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .ok {
    margin: 0;
    color: #3fb950;
    font-size: 0.85rem;
  }
  .finding {
    border: 1px solid var(--color-border);
    border-left-width: 3px;
    border-radius: 6px;
    padding: var(--space-3);
  }
  .finding p {
    margin: var(--space-2) 0 0;
    font-size: 0.8rem;
    line-height: 1.5;
    color: var(--color-text-muted);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    font-size: 0.85rem;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.8rem;
  }
  .tag {
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    border: 1px solid var(--color-border);
  }
  /* Severity shown by the tag text and the left border weight/colour. */
  .finding.warn {
    border-left-color: #d29922;
  }
  .finding.require_ack {
    border-left-color: #ff7b72;
  }
  .finding.block {
    border-left-color: #f85149;
    background: #2d1513;
  }
  .finding.clamp {
    border-left-color: #58a6ff;
  }
</style>
