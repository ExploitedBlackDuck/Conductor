<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import { runControls } from "../stores/runControls";
  import { run } from "../stores/run";
  import ChangeSetPreview from "./ChangeSetPreview.svelte";

  export let preview: app.PreviewDTO | null;

  let acknowledged = false;

  $: requiresAck = preview?.requiresAck ?? false;
  $: hasError = !!preview?.error;
  $: active = $runControls.operationId !== null;
  $: changeSet = $runControls.changeSet;
  // ADR-0015: a destructive run can only proceed once it has been previewed
  // (the change set is shown) and that change set is acknowledged.
  $: previewed = changeSet !== null;
  $: canRun =
    !!preview &&
    !hasError &&
    (!requiresAck || (previewed && acknowledged)) &&
    !$runControls.busy &&
    !active;

  // Return the control to Start once the daemon reports the work is done.
  $: if ($run) runControls.clearIfDone($run.activeJobs);

  // A change to the selection (reflected in a fresh impact preview) invalidates
  // any shown change set, so the operator must re-preview before confirming.
  $: preview, runControls.clearPreview();

  function onAck(e: Event) {
    acknowledged = (e.target as HTMLInputElement).checked;
  }
</script>

<div class="controls">
  {#if requiresAck && !active}
    {#if changeSet}
      <ChangeSetPreview {changeSet} />
      <label class="ack">
        <input type="checkbox" checked={acknowledged} on:change={onAck} />
        <span>I have reviewed the {changeSet.deleteCount} deletion{changeSet.deleteCount === 1 ? "" : "s"} above and want to proceed.</span>
      </label>
    {:else}
      <button class="preview" on:click={() => runControls.previewChanges()} disabled={!preview || hasError || $runControls.busy}>
        {$runControls.busy ? "Previewing…" : "Preview changes (dry-run)"}
      </button>
      <span class="hint">A destructive operation must be previewed before it can run.</span>
    {/if}
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
  .preview {
    background: #1f6feb;
    align-self: flex-start;
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
