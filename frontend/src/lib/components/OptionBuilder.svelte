<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import { builder } from "../stores/builder";
  import OptionRow from "./OptionRow.svelte";

  export let categories: app.CategoryDTO[];

  const selectionStore = builder.selection;
  $: kind = $selectionStore.kind;

  let query = "";

  // An option applies to the current kind unless it restricts to other kinds.
  // Guard against a null `kinds` (a nil Go slice serializing as null) so the
  // builder never crashes into a blank panel.
  function appliesToKind(opt: app.OptionDTO, k: string): boolean {
    const kinds = opt.kinds ?? [];
    return kinds.length === 0 || kinds.includes(k);
  }
  function matchesQuery(opt: app.OptionDTO, q: string): boolean {
    if (q.trim() === "") return true;
    const needle = q.toLowerCase();
    return opt.flag.toLowerCase().includes(needle) || opt.summary.toLowerCase().includes(needle);
  }

  // Recompute the visible groups whenever the catalog, kind, or query change —
  // passing kind/query as args so Svelte tracks them as dependencies.
  function groupsFor(cats: app.CategoryDTO[], k: string, q: string) {
    return cats
      .map((c) => ({ name: c.name, opts: (c.options ?? []).filter((o) => appliesToKind(o, k) && matchesQuery(o, q)) }))
      .filter((g) => g.opts.length > 0);
  }
  $: groups = groupsFor(categories, kind, query);
</script>

<div class="builder">
  <input class="search" type="search" placeholder="Search options…" bind:value={query} />

  {#each groups as group (group.name)}
    <section class="category">
      <h3>{group.name}</h3>
      <div class="options">
        {#each group.opts as opt (opt.flag)}
          <OptionRow option={opt} />
        {/each}
      </div>
    </section>
  {/each}

  {#if groups.length === 0}
    <p class="empty">
      {#if query.trim()}No options match “{query}”.{:else}No options apply to a {kind}.{/if}
    </p>
  {/if}
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
  .empty {
    color: var(--color-text-muted);
    font-size: 0.85rem;
    margin: 0;
  }
</style>
