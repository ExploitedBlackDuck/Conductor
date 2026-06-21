<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import { AcquireRclone, Onboarding } from "../../../wailsjs/go/app/App";
  import { status } from "../stores/status";

  // Shown only for the binary-acquisition degraded states (§7.11.9): a missing
  // or checksum-mismatched rclone routes into the acquisition wizard (ADR-0008).
  export let error: app.ErrorDTO;

  let pinnedVersion = "";
  let busy = false;
  let failure: app.ErrorDTO | null = null;

  $: missing = error.code === "ERR_RCLONE_BINARY_MISSING";
  $: mismatch = error.code === "ERR_RCLONE_BINARY_CHECKSUM";

  onMount(async () => {
    try {
      pinnedVersion = (await Onboarding()).pinnedVersion;
    } catch {
      // binding unavailable outside the webview
    }
  });

  async function acquire() {
    busy = true;
    failure = null;
    try {
      const err = await AcquireRclone();
      if (err) {
        failure = err;
      } else {
        await status.poll(); // the daemon should now be up; refresh the banner away
      }
    } catch {
      // binding unavailable
    }
    busy = false;
  }
</script>

<div class="banner" class:mismatch>
  <div class="text">
    {#if mismatch}
      <strong>rclone failed its integrity check.</strong>
      The binary doesn't match the pinned <code>{pinnedVersion}</code>. Re-download a verified copy to continue.
    {:else}
      <strong>rclone {pinnedVersion} isn't installed yet.</strong>
      Conductor needs the pinned, checksum-verified binary before it can run operations.
    {/if}
  </div>
  <button on:click={acquire} disabled={busy}>
    {busy ? "Downloading & verifying…" : missing ? `Download rclone ${pinnedVersion}` : "Re-download verified rclone"}
  </button>
  {#if failure}
    <p class="err" role="alert"><strong>{failure.code}</strong> — {failure.message}</p>
  {/if}
</div>

<style>
  .banner {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    border: 1px solid #5c4a16;
    background: #2b2310;
    color: #f0c674;
    border-radius: 10px;
    padding: var(--space-3) var(--space-4);
  }
  .banner.mismatch {
    border-color: #5c2b29;
    background: #2b1414;
    color: #ffb3ab;
  }
  .text {
    flex: 1;
    min-width: 16rem;
    font-size: 0.88rem;
    line-height: 1.4;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.82rem;
  }
  button {
    border: none;
    border-radius: 6px;
    padding: 0.5rem 0.9rem;
    background: #238636;
    color: #fff;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .err {
    width: 100%;
    margin: var(--space-2) 0 0;
    color: #ffb3ab;
    font-size: 0.8rem;
  }
</style>
