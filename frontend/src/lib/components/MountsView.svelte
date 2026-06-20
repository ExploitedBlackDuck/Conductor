<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import { mounts, mountsError, refreshMounts, doMount, doUnmount } from "../stores/mounts";

  let fs = "";
  let mountPoint = "";
  let actionError: app.ErrorDTO | null = null;
  let busy = false;

  onMount(refreshMounts);

  async function mount() {
    if (!fs || !mountPoint) return;
    busy = true;
    actionError = await doMount(fs, mountPoint, "");
    busy = false;
    if (!actionError) {
      fs = "";
      mountPoint = "";
    }
  }
  async function unmount(mp: string) {
    busy = true;
    actionError = await doUnmount(mp);
    busy = false;
  }
</script>

<div class="mounts">
  <section class="card">
    <h2>Mount a remote</h2>
    <div class="form">
      <label>
        <span>Remote</span>
        <input type="text" placeholder="remote:path" bind:value={fs} />
      </label>
      <label>
        <span>Mount point</span>
        <input type="text" placeholder="/local/mount/dir" bind:value={mountPoint} />
      </label>
      <button on:click={mount} disabled={busy || !fs || !mountPoint}>Mount</button>
    </div>
    {#if actionError}
      <p class="err" role="alert"><strong>{actionError.code}</strong> — {actionError.message}</p>
    {/if}
  </section>

  <section class="card">
    <h2>Active mounts</h2>
    {#if $mountsError}
      <p class="err" role="alert"><strong>{$mountsError.code}</strong> — {$mountsError.message}</p>
    {/if}
    {#if $mounts.length === 0}
      <p class="muted">No active mounts.</p>
    {:else}
      <ul class="list">
        {#each $mounts as m (m.mountPoint)}
          <li>
            <div class="info">
              <code class="fs">{m.fs}</code>
              <span class="arrow">→</span>
              <code class="mp">{m.mountPoint}</code>
            </div>
            <button class="unmount" on:click={() => unmount(m.mountPoint)} disabled={busy}>Unmount</button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .mounts {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    max-width: 44rem;
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
    grid-template-columns: 1fr 1fr auto;
    gap: var(--space-3);
    align-items: end;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  label span {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  input {
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.45rem 0.55rem;
    font-size: 0.9rem;
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
  .unmount {
    background: #b62324;
  }
  .list {
    list-style: none;
    margin: 0;
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
