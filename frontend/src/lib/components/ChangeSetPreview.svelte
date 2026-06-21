<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";

  // The parsed dry-run preview (ADR-0015). Deletes are the dangerous, always-
  // enumerated kind and are shown first and visually distinct.
  export let changeSet: app.ChangeSetDTO;

  $: empty = changeSet.createCount + changeSet.updateCount + changeSet.deleteCount === 0;
</script>

<div class="preview">
  <div class="summary">
    <span class="chip create">+{changeSet.createCount} create{changeSet.createCount === 1 ? "" : "s"}</span>
    <span class="chip update">~{changeSet.updateCount} update{changeSet.updateCount === 1 ? "" : "s"}</span>
    <span class="chip delete" class:hot={changeSet.deleteCount > 0}>
      −{changeSet.deleteCount} delete{changeSet.deleteCount === 1 ? "" : "s"}
    </span>
    {#if changeSet.truncated}
      <span class="chip muted">list truncated — counts exact, deletes complete</span>
    {/if}
  </div>

  {#if empty}
    <p class="muted">This dry-run would change nothing.</p>
  {:else}
    {#if changeSet.deletes.length > 0}
      <section class="group">
        <h4 class="delete">Will be deleted at the destination</h4>
        <ul>
          {#each changeSet.deletes as f (f.path)}
            <li><code>{f.path}</code></li>
          {/each}
        </ul>
      </section>
    {/if}
    {#if changeSet.updates.length > 0}
      <section class="group">
        <h4 class="update">Will be overwritten</h4>
        <ul>
          {#each changeSet.updates as f (f.path)}
            <li><code>{f.path}</code></li>
          {/each}
        </ul>
      </section>
    {/if}
    {#if changeSet.creates.length > 0}
      <section class="group">
        <h4 class="create">Will be created</h4>
        <ul>
          {#each changeSet.creates as f (f.path)}
            <li><code>{f.path}</code></li>
          {/each}
        </ul>
      </section>
    {/if}
  {/if}
</div>

<style>
  .preview {
    border: 1px solid var(--color-border);
    border-radius: 8px;
    background: var(--color-bg);
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .summary {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .chip {
    font-size: 0.78rem;
    font-weight: 600;
    border-radius: 5px;
    padding: 0.1rem 0.45rem;
    border: 1px solid var(--color-border);
  }
  .chip.create {
    color: #7ee787;
  }
  .chip.update {
    color: #f0c674;
  }
  .chip.delete {
    color: var(--color-text-muted);
  }
  .chip.delete.hot {
    color: #ffb3ab;
    border-color: #5c2b29;
    background: #2b1414;
  }
  .chip.muted {
    color: var(--color-text-muted);
    font-weight: 400;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  h4 {
    margin: 0;
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  h4.delete {
    color: #ffb3ab;
  }
  h4.update {
    color: #f0c674;
  }
  h4.create {
    color: #7ee787;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 12rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.8rem;
    color: var(--color-text);
  }
  .muted {
    color: var(--color-text-muted);
    margin: 0;
    font-size: 0.85rem;
  }
</style>
