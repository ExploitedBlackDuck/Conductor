<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import ProfileEditor from "./ProfileEditor.svelte";
  import {
    pairs,
    profiles,
    ceilings,
    pairsError,
    refreshPairs,
    refreshProfiles,
    refreshCeilings,
    savePair,
    removePair,
    runPair,
    setCeiling,
  } from "../stores/pairs";

  // New-pair form.
  let name = "";
  let kind: "bisync" | "sync" = "bisync";
  let path1 = "";
  let path2 = "";
  let profileId = "";

  // New-ceiling form.
  let remote = "";
  let transfers = 0;
  let checkers = 0;
  let bwlimit = "";
  let tpslimit = 0;

  let actionError: app.ErrorDTO | null = null;
  let busy = false;
  // Pair id awaiting a destructive-run confirmation (§7.4).
  let confirming: string | null = null;

  onMount(() => {
    void refreshPairs();
    void refreshProfiles();
    void refreshCeilings();
  });

  async function addPair() {
    if (!name || !path1 || !path2) return;
    busy = true;
    // A stable-ish client id; the backend treats SavePair as upsert by id.
    const id = `${kind}-${name}-${path1}-${path2}`.replace(/[^a-zA-Z0-9_-]/g, "_");
    actionError = await savePair({ id, name, kind, path1, path2, profileId });
    busy = false;
    if (!actionError) {
      name = "";
      path1 = "";
      path2 = "";
      profileId = "";
    }
  }

  async function run(p: app.PairDTO, acknowledged: boolean) {
    busy = true;
    actionError = null;
    const res = await runPair(p.id, acknowledged);
    busy = false;
    if (res.error) {
      if (res.error.code === "ERR_DESTRUCTIVE_NOT_CONFIRMED") {
        confirming = p.id; // surface an explicit confirm step
      } else {
        actionError = res.error;
      }
      return;
    }
    confirming = null;
  }

  async function del(id: string) {
    busy = true;
    actionError = await removePair(id);
    busy = false;
  }

  async function addCeiling() {
    if (!remote) return;
    busy = true;
    actionError = await setCeiling({ remote, transfers, checkers, bwlimit, tpslimit });
    busy = false;
    if (!actionError) {
      remote = "";
      transfers = 0;
      checkers = 0;
      bwlimit = "";
      tpslimit = 0;
    }
  }
</script>

<div class="pairs">
  <section class="card">
    <h2>Save a sync / bisync pair</h2>
    <div class="form">
      <label>
        <span>Name</span>
        <input type="text" placeholder="laptop ↔ drive" bind:value={name} />
      </label>
      <label>
        <span>Kind</span>
        <select bind:value={kind}>
          <option value="bisync">bisync</option>
          <option value="sync">sync</option>
        </select>
      </label>
      <label>
        <span>Path 1</span>
        <input type="text" placeholder="/home/me" bind:value={path1} />
      </label>
      <label>
        <span>Path 2</span>
        <input type="text" placeholder="gdrive:backup" bind:value={path2} />
      </label>
      <label>
        <span>Profile</span>
        <select bind:value={profileId}>
          <option value="">(none)</option>
          {#each $profiles as pr (pr.id)}
            <option value={pr.id}>{pr.name}</option>
          {/each}
        </select>
      </label>
      <button on:click={addPair} disabled={busy || !name || !path1 || !path2}>Save</button>
    </div>
    {#if actionError}
      <p class="err" role="alert"><strong>{actionError.code}</strong> — {actionError.message}</p>
    {/if}
  </section>

  <section class="card">
    <h2>Saved pairs</h2>
    {#if $pairsError}
      <p class="err" role="alert"><strong>{$pairsError.code}</strong> — {$pairsError.message}</p>
    {/if}
    {#if $pairs.length === 0}
      <p class="muted">No saved pairs yet.</p>
    {:else}
      <ul class="list">
        {#each $pairs as p (p.id)}
          <li>
            <div class="info">
              <span class="pname">{p.name}</span>
              <span class="kind">{p.kind}</span>
              <code class="ep">{p.path1}</code>
              <span class="arrow">{p.kind === "bisync" ? "↔" : "→"}</span>
              <code class="ep">{p.path2}</code>
              {#if !p.hasRun}
                <span class="badge dry" title="A new pair's first run is a dry-run">first run: dry-run</span>
              {/if}
            </div>
            <div class="actions">
              {#if confirming === p.id}
                <span class="confirm-note">Destructive — confirm?</span>
                <button class="confirm" on:click={() => run(p, true)} disabled={busy}>Confirm run</button>
                <button class="ghost" on:click={() => (confirming = null)} disabled={busy}>Cancel</button>
              {:else}
                <button on:click={() => run(p, false)} disabled={busy}>
                  {p.hasRun ? "Run" : "Dry-run"}
                </button>
                <button class="del" on:click={() => del(p.id)} disabled={busy}>Delete</button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <ProfileEditor />

  <section class="card">
    <h2>Per-remote governance ceilings</h2>
    <p class="muted small">
      Conservative caps a remote's transfers cannot exceed; tightening these is the
      safe default and going faster is an explicit, recorded choice (ADR-0013).
    </p>
    <div class="form ceil">
      <label>
        <span>Remote</span>
        <input type="text" placeholder="s3" bind:value={remote} />
      </label>
      <label>
        <span>Transfers</span>
        <input type="number" min="0" bind:value={transfers} />
      </label>
      <label>
        <span>Checkers</span>
        <input type="number" min="0" bind:value={checkers} />
      </label>
      <label>
        <span>Bwlimit</span>
        <input type="text" placeholder="10M" bind:value={bwlimit} />
      </label>
      <label>
        <span>Tps limit</span>
        <input type="number" min="0" bind:value={tpslimit} />
      </label>
      <button on:click={addCeiling} disabled={busy || !remote}>Set</button>
    </div>
    {#if $ceilings.length > 0}
      <ul class="list">
        {#each $ceilings as c (c.remote)}
          <li>
            <div class="info">
              <code class="ep">{c.remote}</code>
              <span class="caps">
                transfers {c.transfers || "—"} · checkers {c.checkers || "—"} ·
                bw {c.bwlimit || "—"} · tps {c.tpslimit || "—"}
              </span>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .pairs {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    max-width: 52rem;
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
  .form {
    display: grid;
    grid-template-columns: 1.4fr 0.8fr 1.4fr 1.4fr 1fr auto;
    gap: var(--space-3);
    align-items: end;
  }
  .form.ceil {
    grid-template-columns: 1.2fr 0.8fr 0.8fr 0.8fr 0.8fr auto;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-width: 0;
  }
  label span {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  input,
  select {
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.45rem 0.55rem;
    font-size: 0.9rem;
    min-width: 0;
  }
  button {
    border: none;
    border-radius: 6px;
    padding: 0.5rem 0.9rem;
    background: #238636;
    color: #fff;
    font-weight: 600;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .del {
    background: #b62324;
  }
  .confirm {
    background: #b62324;
  }
  .ghost {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
  }
  .list {
    list-style: none;
    margin: var(--space-3) 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .list li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: var(--space-3);
  }
  .info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    flex-wrap: wrap;
  }
  .pname {
    font-weight: 600;
  }
  .kind {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
    border-radius: 4px;
    padding: 0.05rem 0.35rem;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.82rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .arrow {
    color: var(--color-text-muted);
  }
  .badge.dry {
    background: #3b2f12;
    color: #f0c674;
    font-size: 0.7rem;
    border-radius: 4px;
    padding: 0.05rem 0.4rem;
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    flex-shrink: 0;
  }
  .confirm-note {
    color: #ffb3ab;
    font-size: 0.78rem;
  }
  .caps {
    color: var(--color-text-muted);
    font-size: 0.82rem;
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
  .small {
    font-size: 0.8rem;
    margin-bottom: var(--space-3);
  }
</style>
