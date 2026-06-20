<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import { runControls } from "../stores/runControls";
  import { run } from "../stores/run";

  export let preview: app.PreviewDTO | null;

  let acknowledged = false;

  $: requiresAck = preview?.requiresAck ?? false;
  $: hasError = !!preview?.error;
  $: active = $runControls.operationId !== null;
  $: canRun = !!preview && !hasError && (!requiresAck || acknowledged) && !$runControls.busy && !active;

  // Return the control to Start once the daemon reports the work is done.
  $: if ($run) runControls.clearIfDone($run.activeJobs);

  function onAck(e: Event) {
    acknowledged = (e.target as HTMLInputElement).checked;
  }
</script>

<div class="controls">
  {#if requiresAck && !active}
    <label class="ack">
      <input type="checkbox" checked={acknowledged} on:change={onAck} />
      <span>I understand this operation can change or delete data and want to proceed.</span>
    </label>
  {/if}

  {#if active}
    <button class="stop" on:click={() => runControls.stop()} disabled={$runControls.busy}>
      ■ Stop
    </button>
    <span class="hint">Running — watch the Dashboard for live progress.</span>
  {:else}
    <button class="start" on:click={() => runControls.start(acknowledged)} disabled={!canRun}>
      ▶ Start {preview?.kind ?? ""}
    </button>
  {/if}

  {#if $runControls.error}
    <p class="err" role="alert"><strong>{$runControls.error.code}</strong> — {$runControls.error.message}</p>
  {/if}
</div>

<style>
  .controls {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  button {
    border: none;
    border-radius: 8px;
    padding: 0.6rem 1rem;
    font-size: 0.95rem;
    font-weight: 600;
    cursor: pointer;
    color: #fff;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .start {
    background: #238636;
  }
  .stop {
    background: #b62324;
  }
  .ack {
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
    font-size: 0.82rem;
    line-height: 1.4;
    color: #ffb3ab;
    border: 1px solid #5c2b29;
    border-radius: 8px;
    padding: var(--space-3);
  }
  .ack input {
    margin-top: 0.2rem;
  }
  .hint {
    font-size: 0.8rem;
    color: var(--color-text-muted);
  }
  .err {
    margin: 0;
    color: #ffb3ab;
    font-size: 0.82rem;
  }
</style>
