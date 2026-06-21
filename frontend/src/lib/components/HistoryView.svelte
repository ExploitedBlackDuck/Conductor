<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import { humanBytes } from "../format";
  import {
    operations,
    historyError,
    loadHistory,
    detail,
    exportHistory,
    clearHistory,
    type Filter,
  } from "../stores/history";
  import RiskBadge from "./RiskBadge.svelte";

  let filter: Filter = { kind: "recent" };
  let remoteInput = "";
  let busy = false;
  let actionError: app.ErrorDTO | null = null;
  let confirmingClear = false;

  // Expanded operation detail, keyed by operation id.
  let openId: string | null = null;
  let openDetail: app.OperationDetailDTO | null = null;

  onMount(() => void loadHistory(filter));

  async function apply(f: Filter) {
    filter = f;
    openId = null;
    await loadHistory(f);
  }

  async function toggleDetail(id: string) {
    if (openId === id) {
      openId = null;
      return;
    }
    openId = id;
    openDetail = await detail(id);
  }

  async function doExport(format: "json" | "csv") {
    busy = true;
    actionError = await exportHistory(format, filter.kind === "remote" ? filter.remote : "");
    busy = false;
  }

  async function doClear() {
    busy = true;
    actionError = await clearHistory();
    busy = false;
    confirmingClear = false;
  }

  function shortTime(iso: string): string {
    return iso ? iso.replace("T", " ").replace(/[+Z].*$/, "") : "—";
  }
</script>

<div class="history">
  <section class="card">
    <div class="toolbar">
      <div class="filters">
        <button class:active={filter.kind === "recent"} on:click={() => apply({ kind: "recent" })}>Recent</button>
        <button class:active={filter.kind === "destructive"} on:click={() => apply({ kind: "destructive" })}>
          Destructive
        </button>
        <div class="remote">
          <input
            type="text"
            placeholder="remote"
            bind:value={remoteInput}
            on:keydown={(e) => e.key === "Enter" && remoteInput && apply({ kind: "remote", remote: remoteInput })}
          />
          <button on:click={() => remoteInput && apply({ kind: "remote", remote: remoteInput })}>By remote</button>
        </div>
      </div>
      <div class="exports">
        <button class="ghost" on:click={() => doExport("json")} disabled={busy}>Export JSON</button>
        <button class="ghost" on:click={() => doExport("csv")} disabled={busy}>Export CSV</button>
        {#if confirmingClear}
          <button class="del" on:click={doClear} disabled={busy}>Confirm clear</button>
          <button class="ghost" on:click={() => (confirmingClear = false)} disabled={busy}>Cancel</button>
        {:else}
          <button class="del" on:click={() => (confirmingClear = true)} disabled={busy}>Clear history</button>
        {/if}
      </div>
    </div>
    {#if actionError}
      <p class="err" role="alert"><strong>{actionError.code}</strong> — {actionError.message}</p>
    {/if}
    {#if $historyError}
      <p class="err" role="alert"><strong>{$historyError.code}</strong> — {$historyError.message}</p>
    {/if}
  </section>

  <section class="card">
    {#if $operations.length === 0}
      <p class="muted">No operations yet.</p>
    {:else}
      <table>
        <thead>
          <tr>
            <th>Kind</th>
            <th>Source → Destination</th>
            <th>Started</th>
            <th class="num">Bytes</th>
            <th class="num">Files</th>
            <th>Result</th>
          </tr>
        </thead>
        <tbody>
          {#each $operations as op (op.id)}
            <tr class="row" class:open={openId === op.id} on:click={() => toggleDetail(op.id)}>
              <td>
                <span class="kind">{op.kind}</span>
                {#if op.destructive}<span class="dbadge" title="Destructive operation">!</span>{/if}
              </td>
              <td class="eps"><code>{op.src}</code> <span class="arrow">→</span> <code>{op.dst}</code></td>
              <td class="time">{shortTime(op.startedAt)}</td>
              <td class="num">{humanBytes(op.bytesMoved)}</td>
              <td class="num">{op.filesMoved}</td>
              <td><span class="result {op.result}">{op.result}</span></td>
            </tr>
            {#if openId === op.id && openDetail}
              <tr class="detail">
                <td colspan="6">
                  {#if (openDetail.options ?? []).length === 0}
                    <span class="muted">No options recorded.</span>
                  {:else}
                    <div class="opts">
                      {#each openDetail.options as o (o.flag)}
                        <span class="opt">
                          <code>{o.flag}</code>{#if o.value !== "true"}=<code>{o.value}</code>{/if}
                          <RiskBadge risk={o.risk} />
                          {#if o.acknowledged}<span class="ack">acknowledged</span>{/if}
                        </span>
                      {/each}
                    </div>
                  {/if}
                  <div class="meta">
                    rclone {openDetail.operation.rcloneVersion} · id {openDetail.operation.id}{#if openDetail.operation.serverSide} · <span class="ss">server-side</span>{/if}
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</div>

<style>
  .history {
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
  .toolbar {
    display: flex;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .filters,
  .exports,
  .remote {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  button {
    border: none;
    border-radius: 6px;
    padding: 0.4rem 0.75rem;
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    cursor: pointer;
    font-size: 0.85rem;
  }
  button.active {
    background: var(--color-accent, #1f6feb);
    color: #fff;
    border-color: transparent;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .ghost {
    color: var(--color-text-muted);
  }
  .del {
    color: #ffb3ab;
    border-color: #5c2b2b;
  }
  input {
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.35rem 0.5rem;
    font-size: 0.85rem;
    width: 8rem;
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
  }
  .row {
    cursor: pointer;
  }
  .row:hover {
    background: var(--color-bg);
  }
  .row.open {
    background: var(--color-bg);
  }
  .kind {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .dbadge {
    color: #ff7b72;
    font-weight: 700;
    margin-left: 0.25rem;
  }
  .eps {
    min-width: 0;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.8rem;
  }
  .arrow {
    color: var(--color-text-muted);
  }
  .time {
    color: var(--color-text-muted);
    white-space: nowrap;
  }
  .result {
    font-size: 0.72rem;
    text-transform: uppercase;
    padding: 0.05rem 0.4rem;
    border-radius: 4px;
    border: 1px solid var(--color-border);
  }
  .result.success {
    color: #7ee787;
  }
  .result.failed {
    color: #ffb3ab;
  }
  .result.cancelled {
    color: #f0c674;
  }
  .detail td {
    background: var(--color-bg);
  }
  .opts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .opt {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.1rem 0.4rem;
  }
  .ack {
    font-size: 0.65rem;
    text-transform: uppercase;
    color: #ff7b72;
  }
  .meta {
    margin-top: var(--space-2);
    color: var(--color-text-muted);
    font-size: 0.75rem;
  }
  .ss {
    color: #7ee787;
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
