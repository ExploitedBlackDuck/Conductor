<script lang="ts">
  // A keyboard-first command palette (§7.13): Cmd/Ctrl+K opens it, type to
  // filter, arrows to move, Enter to run, Esc to close. It only navigates and
  // triggers actions that themselves enforce their gates — no shortcut bypasses
  // the destructive-op confirm (§7.4).
  export let commands: { id: string; label: string; hint?: string; run: () => void }[];

  let open = false;
  let query = "";
  let selected = 0;
  let input: HTMLInputElement | undefined;

  $: filtered = commands.filter((c) => c.label.toLowerCase().includes(query.trim().toLowerCase()));
  $: if (selected >= filtered.length) selected = Math.max(0, filtered.length - 1);

  function onWindowKey(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      open ? close() : openPalette();
    } else if (open && e.key === "Escape") {
      close();
    }
  }

  function openPalette() {
    open = true;
    query = "";
    selected = 0;
    // Focus after the input renders.
    queueMicrotask(() => input?.focus());
  }
  function close() {
    open = false;
  }
  function exec(c: { run: () => void }) {
    close();
    c.run();
  }

  function onInputKey(e: KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selected = Math.min(selected + 1, filtered.length - 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selected = Math.max(selected - 1, 0);
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (filtered[selected]) exec(filtered[selected]);
    }
  }
</script>

<svelte:window on:keydown={onWindowKey} />

{#if open}
  <div class="wrap">
    <button class="backdrop" aria-label="Close command palette" on:click={close}></button>
    <div class="palette" role="dialog" aria-label="Command palette" aria-modal="true">
      <!-- svelte-ignore a11y-autofocus -->
      <input
        bind:this={input}
        type="text"
        placeholder="Go to a view or action…"
        bind:value={query}
        on:keydown={onInputKey}
      />
      <ul>
        {#each filtered as c, i (c.id)}
          <li>
            <button class="item" class:sel={i === selected} on:click={() => exec(c)} on:mouseenter={() => (selected = i)}>
              <span>{c.label}</span>
              {#if c.hint}<span class="hint">{c.hint}</span>{/if}
            </button>
          </li>
        {/each}
        {#if filtered.length === 0}
          <li class="empty">No matches</li>
        {/if}
      </ul>
    </div>
  </div>
{/if}

<style>
  .wrap {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
  }
  .backdrop {
    position: absolute;
    inset: 0;
    border: none;
    background: rgba(0, 0, 0, 0.45);
    cursor: default;
  }
  .palette {
    position: relative;
    width: min(34rem, 92vw);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.5);
    overflow: hidden;
  }
  input {
    width: 100%;
    box-sizing: border-box;
    border: none;
    border-bottom: 1px solid var(--color-border);
    background: transparent;
    color: var(--color-text);
    padding: 0.85rem 1rem;
    font-size: 0.95rem;
    outline: none;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: var(--space-2);
    max-height: 22rem;
    overflow-y: auto;
  }
  .item {
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    background: none;
    border: none;
    color: var(--color-text);
    text-align: left;
    padding: 0.55rem 0.7rem;
    border-radius: 7px;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .item.sel {
    background: var(--color-bg);
  }
  .hint {
    color: var(--color-text-muted);
    font-size: 0.75rem;
  }
  .empty {
    color: var(--color-text-muted);
    padding: 0.6rem 0.7rem;
    font-size: 0.88rem;
  }
</style>
