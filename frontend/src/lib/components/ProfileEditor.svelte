<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../../wailsjs/go/models";
  import { catalog, loadCatalog } from "../stores/catalog";
  import { profiles, refreshProfiles, saveProfile, removeProfile } from "../stores/pairs";
  import RiskBadge from "./RiskBadge.svelte";

  // A profile is a named option set (§7.5). Its kind scopes which catalog
  // options apply; the editor writes flag→value pairs the backend re-validates.
  type Kind = "copy" | "sync" | "move" | "bisync";
  let name = "";
  let kind: Kind = "sync";
  // Local, uncommitted selection: scalars and list values keyed by flag.
  let single: Record<string, string> = {};
  let multi: Record<string, string[]> = {};

  let actionError: app.ErrorDTO | null = null;
  let busy = false;

  onMount(() => {
    void loadCatalog();
    void refreshProfiles();
  });

  // Options applicable to the chosen kind: an empty kinds list means all kinds.
  $: options = ($catalog?.categories ?? [])
    .flatMap((c) => c.options)
    .filter((o) => !o.kinds || o.kinds.length === 0 || o.kinds.includes(kind));

  function setSingle(flag: string, value: string) {
    if (value === "" || value === "false") {
      const { [flag]: _drop, ...rest } = single;
      single = rest;
    } else {
      single = { ...single, [flag]: value };
    }
  }
  function setList(flag: string, raw: string) {
    const vals = raw.split("\n").map((v) => v.trim()).filter((v) => v !== "");
    if (vals.length === 0) {
      const { [flag]: _drop, ...rest } = multi;
      multi = rest;
    } else {
      multi = { ...multi, [flag]: vals };
    }
  }

  function reset() {
    name = "";
    single = {};
    multi = {};
  }

  async function save() {
    if (!name) return;
    busy = true;
    // Flatten to flag/value rows; a list flag emits one row per value.
    const opts: app.ProfileOptionDTO[] = [];
    for (const [flag, value] of Object.entries(single)) opts.push({ flag, value } as app.ProfileOptionDTO);
    for (const [flag, values] of Object.entries(multi)) {
      for (const value of values) opts.push({ flag, value } as app.ProfileOptionDTO);
    }
    const id = `${kind}-${name}`.replace(/[^a-zA-Z0-9_-]/g, "_");
    actionError = await saveProfile({ id, name, kind, options: opts });
    busy = false;
    if (!actionError) reset();
  }

  async function del(id: string) {
    busy = true;
    actionError = await removeProfile(id);
    busy = false;
  }

  $: selectedCount = Object.keys(single).length + Object.keys(multi).length;
</script>

<section class="card">
  <h2>Option profiles</h2>
  <p class="muted small">
    A named, reusable option set a saved pair can apply (§7.5). The backend
    re-validates every option when the pair runs.
  </p>

  {#if $profiles.length > 0}
    <ul class="list">
      {#each $profiles as pr (pr.id)}
        <li>
          <div class="info">
            <span class="pname">{pr.name}</span>
            <span class="kind">{pr.kind}</span>
            <span class="caps">{(pr.options ?? []).length} option{(pr.options ?? []).length === 1 ? "" : "s"}</span>
          </div>
          <button class="del" on:click={() => del(pr.id)} disabled={busy}>Delete</button>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="newform">
    <div class="head">
      <label>
        <span>Name</span>
        <input type="text" placeholder="safe copy" bind:value={name} />
      </label>
      <label>
        <span>Kind</span>
        <select bind:value={kind}>
          <option value="sync">sync</option>
          <option value="bisync">bisync</option>
          <option value="copy">copy</option>
          <option value="move">move</option>
        </select>
      </label>
      <span class="count">{selectedCount} selected</span>
      <button on:click={save} disabled={busy || !name}>Save profile</button>
    </div>

    {#if $catalog === null}
      <p class="muted small">Option catalog unavailable (running outside the app).</p>
    {:else}
      <div class="opts">
        {#each options as o (o.flag)}
          <div class="opt">
            <div class="optlabel">
              <code>{o.flag}</code>
              <RiskBadge risk={o.risk} />
              <span class="osum">{o.summary}</span>
            </div>
            <div class="optctl">
              {#if o.type === "bool"}
                <label class="switch">
                  <input
                    type="checkbox"
                    checked={single[o.flag] === "true"}
                    on:change={(e) => setSingle(o.flag, e.currentTarget.checked ? "true" : "false")}
                  />
                </label>
              {:else if o.type === "enum"}
                <select value={single[o.flag] ?? ""} on:change={(e) => setSingle(o.flag, e.currentTarget.value)}>
                  <option value="">(default)</option>
                  {#each o.enum as choice (choice)}
                    <option value={choice}>{choice}</option>
                  {/each}
                </select>
              {:else if o.type === "list"}
                <textarea
                  rows="2"
                  placeholder="one per line"
                  value={(multi[o.flag] ?? []).join("\n")}
                  on:input={(e) => setList(o.flag, e.currentTarget.value)}
                ></textarea>
              {:else}
                <input
                  type={o.type === "int" ? "number" : "text"}
                  placeholder={o.default ? `default: ${o.default}` : ""}
                  value={single[o.flag] ?? ""}
                  on:input={(e) => setSingle(o.flag, e.currentTarget.value)}
                />
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  {#if actionError}
    <p class="err" role="alert"><strong>{actionError.code}</strong> — {actionError.message}</p>
  {/if}
</section>

<style>
  .card {
    border: 1px solid var(--color-border);
    border-radius: 10px;
    background: var(--color-surface);
    padding: var(--space-4);
  }
  .card h2 {
    margin: 0 0 var(--space-2);
    font-size: 0.95rem;
  }
  .head {
    display: flex;
    align-items: end;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  label span {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
  }
  .count {
    font-size: 0.78rem;
    color: var(--color-text-muted);
    margin-left: auto;
  }
  input,
  select,
  textarea {
    background: var(--color-bg);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: 6px;
    padding: 0.4rem 0.5rem;
    font-family: inherit;
    font-size: 0.85rem;
  }
  button {
    border: none;
    border-radius: 6px;
    padding: 0.5rem 0.9rem;
    background: #238636;
    color: #fff;
    font-weight: 600;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .del {
    background: #b62324;
  }
  .opts {
    margin-top: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-height: 22rem;
    overflow-y: auto;
  }
  .opt {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg);
  }
  .optlabel {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    flex-wrap: wrap;
  }
  .osum {
    font-size: 0.8rem;
    color: var(--color-text-muted);
  }
  .optctl {
    flex-shrink: 0;
  }
  .optctl input[type="text"],
  .optctl input[type="number"],
  .optctl select {
    width: 9rem;
  }
  .optctl textarea {
    width: 12rem;
  }
  .switch input {
    width: auto;
  }
  .list {
    list-style: none;
    margin: var(--space-2) 0 var(--space-3);
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .list li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: var(--space-3);
  }
  .info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .pname {
    font-weight: 600;
  }
  .kind {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
    border-radius: 4px;
    padding: 0.05rem 0.35rem;
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.82rem;
  }
  .caps {
    color: var(--color-text-muted);
    font-size: 0.82rem;
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
    margin-bottom: var(--space-3);
  }
</style>
