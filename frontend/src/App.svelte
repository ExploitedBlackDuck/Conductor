<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { status } from "./lib/stores/status";
  import StatusPanel from "./lib/components/StatusPanel.svelte";

  onMount(() => status.start(1000));
  onDestroy(() => status.stop());
</script>

<main class="screen">
  <header class="header">
    <h1 class="title">Conductor</h1>
    <p class="subtitle">rclone control panel</p>
  </header>

  <StatusPanel />
</main>

<style>
  .screen {
    max-width: 56rem;
    margin: 0 auto;
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .header {
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-4);
  }

  .title {
    margin: 0;
    font-size: 1.75rem;
    letter-spacing: -0.02em;
  }

  .subtitle {
    margin: var(--space-2) 0 0;
    color: var(--color-text-muted);
  }
</style>
