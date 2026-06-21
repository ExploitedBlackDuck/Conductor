<script lang="ts">
  import type { app } from "../../../wailsjs/go/models";
  import { builder } from "../stores/builder";
  import RiskBadge from "./RiskBadge.svelte";

  export let option: app.OptionDTO;

  let expanded = false;

  // Current values come from the shared selection store so the row reflects
  // resets and cross-field changes.
  const selectionStore = builder.selection;
  $: selection = $selectionStore;
  $: singleValue = selection.single[option.flag] ?? "";
  $: listValue = (selection.multi[option.flag] ?? []).join("\n");
  $: isBool = option.type === "bool";
  $: isList = option.type === "list";
  $: isEnum = option.type === "enum";
  $: enabled = isList ? (selection.multi[option.flag]?.length ?? 0) > 0 : singleValue !== "" && singleValue !== "false";

  function onBoolChange(e: Event) {
    builder.setSingle(option.flag, (e.target as HTMLInputElement).checked ? "true" : "false");
  }
  function onScalarChange(e: Event) {
    builder.setSingle(option.flag, (e.target as HTMLInputElement | HTMLSelectElement).value);
  }
  function onListChange(e: Event) {
    builder.setMulti(option.flag, (e.target as HTMLTextAreaElement).value.split("\n"));
  }
</script>

<div class="row" class:enabled>
  <div class="head">
    <div class="label">
      <code>{option.flag}</code>
      <RiskBadge risk={option.risk} />
      {#if option.affectsData}
        <span class="affects" title="Affects the data being moved">affects data</span>
      {/if}
    </div>
    <button class="help" type="button" on:click={() => (expanded = !expanded)} aria-expanded={expanded}>
      {expanded ? "Hide" : "Help"}
    </button>
  </div>

  <p class="summary">{option.summary}</p>
  {#if expanded}
    <p class="desc">{option.description}</p>
  {/if}

  <div class="control">
    {#if isBool}
      <label class="switch">
        <input type="checkbox" checked={singleValue === "true"} on:change={onBoolChange} />
        <span>Enabled</span>
      </label>
    {:else if isEnum}
      <select value={singleValue} on:change={onScalarChange}>
        <option value="">(default: {option.default || "none"})</option>
        {#each option.enum ?? [] as choice (choice)}
          <option value={choice}>{choice}</option>
        {/each}
      </select>
    {:else if isList}
      <textarea
        rows="2"
        placeholder="one pattern per line"
        value={listValue}
        on:input={onListChange}
      ></textarea>
    {:else}
      <input
        type={option.type === "int" ? "number" : "text"}
        placeholder={option.default ? `default: ${option.default}` : ""}
        value={singleValue}
        on:input={onScalarChange}
      />
    {/if}
  </div>
</div>

<style>
  .row {
    padding: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    background: var(--color-bg);
  }
  .row.enabled {
    border-color: var(--color-accent);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-2);
  }
  .label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.85rem;
  }
  .affects {
    font-size: 0.65rem;
    text-transform: uppercase;
    color: #ff7b72;
  }
  .help {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-muted);
    border-radius: 6px;
    padding: 0.15rem 0.5rem;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .summary {
    margin: var(--space-2) 0 0;
    font-size: 0.85rem;
    color: var(--color-text-muted);
  }
  .desc {
    margin: var(--space-2) 0 0;
    font-size: 0.8rem;
    line-height: 1.5;
  }
  .control {
    margin-top: var(--space-3);
  }
  input,
  select,
  textarea {
    width: 100%;
    background: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.4rem 0.5rem;
    font-family: inherit;
    font-size: 0.85rem;
  }
  .switch {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-size: 0.85rem;
  }
  .switch input {
    width: auto;
  }
</style>
