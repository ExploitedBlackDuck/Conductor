<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { run } from "../stores/run";
  import { humanBytes, humanRate } from "../format";

  onMount(() => void run.start());
  onDestroy(() => run.stop());

  $: stats = $run;
  $: pct = stats && stats.totalBytes > 0 ? Math.min(100, Math.round((stats.bytes / stats.totalBytes) * 100)) : 0;
  function eta(s: number | undefined): string {
    if (s === undefined || s === null || s <= 0) return "—";
    const m = Math.floor(s / 60);
    const sec = Math.round(s % 60);
    return m > 0 ? `${m}m ${sec}s` : `${sec}s`;
  }
</script>

<div class="dash">
  {#if stats === null}
    <p class="muted">Waiting for the daemon…</p>
  {:else}
    <div class="tiles">
      <div class="tile"><span class="k">Throughput</span><span class="v">{humanRate(stats.speed)}</span></div>
      <div class="tile"><span class="k">Transferred</span><span class="v">{humanBytes(stats.bytes)}</span></div>
      <div class="tile"><span class="k">Active jobs</span><span class="v">{stats.activeJobs}</span></div>
      <div class="tile"><span class="k">ETA</span><span class="v">{eta(stats.etaSeconds ?? undefined)}</span></div>
      <div class="tile" class:bad={stats.errors > 0}><span class="k">Errors</span><span class="v">{stats.errors}</span></div>
    </div>

    {#if stats.totalBytes > 0}
      <div class="overall">
        <div class="bar" role="progressbar" aria-valuenow={pct} aria-valuemin="0" aria-valuemax="100">
          <span style="width: {pct}%"></span>
        </div>
        <span class="pct">{pct}%</span>
      </div>
    {/if}

    <section class="files">
      <h3>Transferring ({stats.transferring.length})</h3>
      {#if stats.transferring.length === 0}
        <p class="muted">No files in flight.</p>
      {:else}
        <ul>
          {#each stats.transferring as f (f.name)}
            <li>
              <div class="frow">
                <span class="fname" title={f.name}>{f.name}</span>
                <span class="fmeta">{humanRate(f.speed)} · {f.percentage}%</span>
              </div>
              <div class="bar small"><span style="width: {f.percentage}%"></span></div>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}
</div>

<style>
  .dash {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
    gap: var(--space-3);
  }
  .tile {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: var(--space-3);
    background: var(--color-surface);
  }
  .tile.bad .v {
    color: #ff7b72;
  }
  .k {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  .v {
    font-size: 1.25rem;
    font-variant-numeric: tabular-nums;
  }
  .overall {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .bar {
    flex: 1;
    height: 0.5rem;
    background: var(--color-border);
    border-radius: 999px;
    overflow: hidden;
  }
  .bar.small {
    height: 0.3rem;
    margin-top: var(--space-2);
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--color-accent);
  }
  .pct {
    font-variant-numeric: tabular-nums;
    font-size: 0.85rem;
    width: 3rem;
    text-align: right;
  }
  .files h3 {
    margin: 0 0 var(--space-2);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-text-muted);
  }
  .files ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .frow {
    display: flex;
    justify-content: space-between;
    gap: var(--space-3);
    font-size: 0.82rem;
  }
  .fname {
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .fmeta {
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .muted {
    color: var(--color-text-muted);
    margin: 0;
  }
</style>
