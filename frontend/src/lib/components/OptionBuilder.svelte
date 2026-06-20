<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import { builder } from "../stores/builder";
  import OptionRow from "./OptionRow.svelte";

  export let categories: app.CategoryDTO[];

  const selectionStore = builder.selection;
  $: kind = $selectionStore.kind;

  let query = "";

  // An option applies to the current kind unless it restricts to other kinds.
  function appliesToKind(opt: app.OptionDTO): boolean {
    return opt.kinds.length === 0 || opt.kinds.includes(kind);
  }
  function matchesQuery(opt: app.OptionDTO): boolean {
    if (query.trim() === "") return true;
    const q = query.toLowerCase();
    return opt.flag.toLowerCase().includes(q) || opt.summary.toLowerCase().includes(q);
  }
  function visibleOptions(cat: app.CategoryDTO): app.OptionDTO[] {
    return cat.options.filter((o) => appliesToKind(o) && matchesQuery(o));
  }
</script>

<div class="builder">
  <input class="search" type="search" placeholder="Search options…" bind:value={query} />

  {#each categories as cat (cat.name)}
    {@const opts = visibleOptions(cat)}
    {#if opts.length > 0}
      <section class="category">
        <h3>{cat.name}</h3>
        <div class="options">
          {#each opts as opt (opt.flag)}
            <OptionRow option={opt} />
          {/each}
        </div>
      </section>
    {/if}
  {/each}
</div>

<style>
  .builder {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .search {
    width: 100%;
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.5rem 0.6rem;
    font-size: 0.9rem;
  }
  .category h3 {
    margin: 0 0 var(--space-2);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-text-muted);
  }
  .options {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
</style>
