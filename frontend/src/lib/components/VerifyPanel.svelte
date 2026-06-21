<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import { verifications, verifyError, refreshVerifications, runVerify } from "../stores/verify";

  let kind: "check" | "cryptcheck" = "check";
  let srcRemote = "";
  let srcPath = "";
  let dstRemote = "";
  let dstPath = "";
  let oneway = false;

  let busy = false;
  let actionError: app.ErrorDTO | null = null;
  let result: app.VerifyResultDTO | null = null;

  onMount(refreshVerifications);

  $: canRun = (!!srcRemote || !!srcPath) && (!!dstRemote || !!dstPath) && !busy;

  async function run() {
    busy = true;
    actionError = null;
    result = null;
    const res = await runVerify(
      kind,
      { remote: srcRemote, path: srcPath } as app.EndpointDTO,
      { remote: dstRemote, path: dstPath } as app.EndpointDTO,
      oneway,
    );
    busy = false;
    if (res.error) {
      actionError = res.error;
    } else {
      result = res;
    }
  }

  function shortTime(iso: string): string {
    return iso ? iso.replace("T", " ").replace(/[+Z].*$/, "") : "—";
  }
</script>

<div class="verify">
  <section class="card">
    <h2>Verify integrity</h2>
    <p class="muted small">
      Compare a source against a destination by hash (or size where hashes aren't
      available). Read-only — a verification never changes a remote (§7.12).
    </p>
    <div class="form">
      <label>
        <span>Kind</span>
        <select bind:value={kind}>
          <option value="check">check</option>
          <option value="cryptcheck">cryptcheck</option>
        </select>
      </label>
      <label>
        <span>Source remote</span>
        <input type="text" placeholder="(local)" bind:value={srcRemote} />
      </label>
      <label>
        <span>Source path</span>
        <input type="text" placeholder="/data or bucket/dir" bind:value={srcPath} />
      </label>
      <label>
        <span>Dest remote</span>
        <input type="text" placeholder="s3" bind:value={dstRemote} />
      </label>
      <label>
        <span>Dest path</span>
        <input type="text" placeholder="bucket/dir" bind:value={dstPath} />
      </label>
      <label class="oneway">
        <input type="checkbox" bind:checked={oneway} />
        <span>One-way (only files on source)</span>
      </label>
      <button on:click={run} disabled={!canRun}>{busy ? "Checking…" : "Verify"}</button>
    </div>
    {#if actionError}
      <p class="err" role="alert"><strong>{actionError.code}</strong> — {actionError.message}</p>
    {/if}
  </section>

  {#if result}
    <section class="card">
      <h3>
        Result:
        <span class="verdict {result.verification.result}">{result.verification.result}</span>
      </h3>
      <div class="counts">
        <span class="count match">{result.verification.match} match</span>
        <span class="count differ" class:hot={result.verification.differ > 0}>{result.verification.differ} differ</span>
        <span class="count missing" class:hot={result.verification.missing > 0}>{result.verification.missing} missing</span>
        <span class="count error" class:hot={result.verification.errorCount > 0}>{result.verification.errorCount} error</span>
      </div>
      {#if result.differ.length > 0}
        <div class="group"><h4 class="differ">Differ</h4><ul>{#each result.differ as p (p)}<li><code>{p}</code></li>{/each}</ul></div>
      {/if}
      {#if result.missingOnDst.length > 0}
        <div class="group"><h4 class="missing">Missing on destination</h4><ul>{#each result.missingOnDst as p (p)}<li><code>{p}</code></li>{/each}</ul></div>
      {/if}
      {#if result.missingOnSrc.length > 0}
        <div class="group"><h4 class="missing">Missing on source</h4><ul>{#each result.missingOnSrc as p (p)}<li><code>{p}</code></li>{/each}</ul></div>
      {/if}
      {#if result.errors.length > 0}
        <div class="group"><h4 class="error">Errors</h4><ul>{#each result.errors as p (p)}<li><code>{p}</code></li>{/each}</ul></div>
      {/if}
    </section>
  {/if}

  <section class="card">
    <h3>Past verifications</h3>
    {#if $verifyError}
      <p class="err" role="alert"><strong>{$verifyError.code}</strong> — {$verifyError.message}</p>
    {/if}
    {#if $verifications.length === 0}
      <p class="muted">No verifications yet.</p>
    {:else}
      <table>
        <thead>
          <tr><th>Kind</th><th>Source → Destination</th><th>When</th><th>Verdict</th></tr>
        </thead>
        <tbody>
          {#each $verifications as v (v.id)}
            <tr>
              <td><span class="kind">{v.kind}</span></td>
              <td class="eps"><code>{v.src}</code> <span class="arrow">→</span> <code>{v.dst}</code></td>
              <td class="time">{shortTime(v.startedAt)}</td>
              <td><span class="verdict {v.result}">{v.result}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </section>
</div>

<style>
  .verify {
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
  .card h2,
  .card h3 {
    margin: 0 0 var(--space-2);
    font-size: 0.95rem;
  }
  .form {
    display: grid;
    grid-template-columns: 0.8fr 1fr 1.2fr 1fr 1.2fr auto;
    gap: var(--space-3);
    align-items: end;
    margin-top: var(--space-3);
  }
  .oneway {
    grid-column: 1 / -2;
    flex-direction: row !important;
    align-items: center;
    gap: var(--space-2);
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
  .oneway input {
    width: auto;
  }
  button {
    border: none;
    border-radius: 6px;
    padding: 0.5rem 0.9rem;
    background: #1f6feb;
    color: #fff;
    font-weight: 600;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .counts {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
    margin-bottom: var(--space-3);
  }
  .count {
    font-size: 0.78rem;
    font-weight: 600;
    border: 1px solid var(--color-border);
    border-radius: 5px;
    padding: 0.1rem 0.45rem;
    color: var(--color-text-muted);
  }
  .count.match {
    color: #7ee787;
  }
  .count.hot {
    color: #ffb3ab;
    border-color: #5c2b29;
  }
  .verdict {
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.05rem 0.4rem;
    border-radius: 4px;
    border: 1px solid var(--color-border);
  }
  .verdict.match {
    color: #7ee787;
  }
  .verdict.mismatch {
    color: #ffb3ab;
    border-color: #5c2b29;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
  h4 {
    margin: 0;
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  h4.differ,
  h4.error {
    color: #ffb3ab;
  }
  h4.missing {
    color: #f0c674;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 12rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
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
  td {
    padding: var(--space-2);
    border-top: 1px solid var(--color-border);
  }
  .kind {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
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
    margin-bottom: var(--space-2);
  }
</style>
