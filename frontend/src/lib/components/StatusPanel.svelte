<script lang="ts">
  import { status } from "../stores/status";
  import { humanBytes, humanRate } from "../format";
</script>

<section class="panel">
  {#if $status === null}
    <p class="muted">Connecting to the rclone daemon…</p>
  {:else}
    <div class="row">
      <span
        class="badge"
        class:up={$status.daemonRunning && !$status.error}
        class:down={!$status.daemonRunning || !!$status.error}
      >
        <span class="dot" aria-hidden="true"></span>
        {$status.daemonRunning ? "Daemon running" : "Daemon stopped"}
      </span>
    </div>

    {#if $status.error}
      <p class="error" role="alert">
        <strong>{$status.error.code}</strong> — {$status.error.message}
      </p>
    {/if}

    <dl class="stats">
      <div><dt>Remotes</dt><dd>{$status.remotes.length}</dd></div>
      <div><dt>Transferred</dt><dd>{humanBytes($status.bytes)}</dd></div>
      <div><dt>Throughput</dt><dd>{humanRate($status.speed)}</dd></div>
      <div><dt>Files</dt><dd>{$status.transfers}</dd></div>
      <div><dt>Errors</dt><dd>{$status.errorsCount}</dd></div>
    </dl>

    {#if $status.remotes.length > 0}
      <ul class="remotes">
        {#each $status.remotes as remote (remote)}
          <li>{remote}</li>
        {/each}
      </ul>
    {:else}
      <p class="muted">No remotes configured.</p>
    {/if}
  {/if}
</section>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .row {
    display: flex;
    align-items: center;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: 999px;
    border: 1px solid var(--color-border);
    font-size: 0.875rem;
  }

  .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    background: var(--color-text-muted);
  }

  /* Status is conveyed by label text and dot shape/colour, not colour alone
     (§8.5 accessibility). */
  .badge.up .dot {
    background: #2ea043;
  }
  .badge.down .dot {
    background: #d29922;
  }

  .error {
    margin: 0;
    padding: var(--space-3);
    border-radius: 8px;
    border: 1px solid #5c2b29;
    background: #2d1513;
    color: #ffb3ab;
    font-size: 0.875rem;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
    gap: var(--space-3);
    margin: 0;
  }

  .stats div {
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: var(--space-3);
    background: var(--color-bg);
  }

  .stats dt {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .stats dd {
    margin: var(--space-2) 0 0;
    font-size: 1.1rem;
    font-variant-numeric: tabular-nums;
  }

  .remotes {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .remotes li {
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: 6px;
  }

  .muted {
    color: var(--color-text-muted);
    margin: 0;
  }
</style>
