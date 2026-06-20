<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { status } from "./lib/stores/status";
  import StatusPanel from "./lib/components/StatusPanel.svelte";
  import OperationBuilder from "./lib/components/OperationBuilder.svelte";

  // Views land as their phases do; only those that are real are shown.
  type View = "transfers" | "status";
  let view: View = "transfers";
  const nav: { id: View; label: string }[] = [
    { id: "transfers", label: "Transfers" },
    { id: "status", label: "Status" },
  ];

  onMount(() => status.start(1000));
  onDestroy(() => status.stop());
</script>

<div class="app">
  <nav class="rail">
    <div class="brand">Conductor</div>
    <ul>
      {#each nav as item (item.id)}
        <li>
          <button class:active={view === item.id} on:click={() => (view = item.id)}>
            {item.label}
          </button>
        </li>
      {/each}
    </ul>
  </nav>

  <main class="content">
    {#if view === "transfers"}
      <OperationBuilder />
    {:else if view === "status"}
      <StatusPanel />
    {/if}
  </main>
</div>

<style>
  .app {
    display: grid;
    grid-template-columns: 12rem 1fr;
    height: 100vh;
  }
  .rail {
    border-right: 1px solid var(--color-border);
    background: var(--color-surface);
    padding: var(--space-4) var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .brand {
    font-weight: 600;
    letter-spacing: -0.01em;
    padding: 0 var(--space-2);
  }
  .rail ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .rail button {
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: var(--color-text-muted);
    padding: var(--space-2) var(--space-3);
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .rail button.active {
    background: var(--color-bg);
    color: var(--color-text);
  }
  .content {
    overflow-y: auto;
    padding: var(--space-6);
  }
</style>
