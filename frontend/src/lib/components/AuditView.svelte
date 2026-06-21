<script lang="ts">
  import { onMount } from "svelte";
  import { audit, loadAudit } from "../stores/history";

  let destructiveOnly = false;

  onMount(loadAudit);

  // Actions that record a destructive confirmation or risk acknowledgement.
  const destructiveActions = new Set([
    "operation.destructive_confirmed",
    "operation.risk_acknowledged",
  ]);

  $: entries = ($audit?.entries ?? []).filter(
    (e) => !destructiveOnly || destructiveActions.has(e.action),
  );
</script>

<div class="audit">
  <section class="card">
    <div class="head">
      <h2>Audit log</h2>
      {#if $audit}
        <span class="indicator" class:intact={$audit.trustworthy} class:broken={!$audit.trustworthy}>
          <span class="dot"></span>
          {#if $audit.trustworthy}
            Chain intact · {$audit.entries.length} entries{#if $audit.headSigned} · signed head verified{/if}
          {:else if !$audit.intact}
            Tampering detected at entry {$audit.brokenAtSeq}
          {:else}
            Signed-head verification failed
          {/if}
        </span>
      {/if}
    </div>
    {#if $audit && !$audit.trustworthy && $audit.reason}
      <p class="err" role="alert">{$audit.reason}</p>
    {/if}
    {#if $audit?.error}
      <p class="err" role="alert"><strong>{$audit.error.code}</strong> — {$audit.error.message}</p>
    {/if}
    <label class="toggle">
      <input type="checkbox" bind:checked={destructiveOnly} />
      <span>Destructive confirmations only</span>
    </label>
  </section>

  <section class="card">
    {#if entries.length === 0}
      <p class="muted">No audit entries.</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th class="num">#</th>
            <th>When</th>
            <th>Action</th>
            <th>Subject</th>
          </tr>
        </thead>
        <tbody>
          {#each entries as e (e.seq)}
            <tr class:destructive={destructiveActions.has(e.action)}>
              <td class="num">{e.seq}</td>
              <td class="time">{e.at.replace("T", " ").replace(/[+Z].*$/, "")}</td>
              <td><code>{e.action}</code></td>
              <td class="subject">{e.subject}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</div>

<style>
  .audit {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .card {
    border: 1px solid var(--color-border);
    border-radius: 10px;
    background: var(--color-surface);
    padding: var(--space-4);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
  }
  .head h2 {
    margin: 0;
    font-size: 0.95rem;
  }
  .indicator {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-size: 0.82rem;
    font-weight: 600;
  }
  .indicator .dot {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
  }
  .indicator.intact {
    color: #7ee787;
  }
  .indicator.intact .dot {
    background: #2ea043;
  }
  .indicator.broken {
    color: #ffb3ab;
  }
  .indicator.broken .dot {
    background: #da3633;
  }
  .toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-3);
    font-size: 0.82rem;
    color: var(--color-text-muted);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }
  th {
    text-align: left;
    color: var(--color-text-muted);
    font-weight: 500;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0 var(--space-2) var(--space-2);
  }
  th.num {
    text-align: right;
  }
  td {
    padding: var(--space-2);
    border-top: 1px solid var(--color-border);
  }
  td.num {
    text-align: right;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-muted);
  }
  tr.destructive code {
    color: #ff7b72;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.8rem;
  }
  .time {
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .subject {
    font-family: var(--font-mono);
    font-size: 0.78rem;
    color: var(--color-text-muted);
  }
  .err {
    margin: var(--space-3) 0 0;
    color: #ffb3ab;
    font-size: 0.82rem;
  }
  .muted {
    color: var(--color-text-muted);
    margin: 0;
  }
</style>
