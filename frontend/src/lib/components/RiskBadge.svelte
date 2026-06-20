<script lang="ts">
  export let risk: string;

  // Meaning is conveyed by the label and a shape glyph as well as colour, never
  // colour alone (§8.5).
  const meta: Record<string, { label: string; glyph: string }> = {
    passive: { label: "Passive", glyph: "○" },
    mutating: { label: "Mutating", glyph: "◐" },
    destructive: { label: "Destructive", glyph: "●" },
  };
  $: m = meta[risk] ?? { label: risk, glyph: "•" };
</script>

<span class="risk {risk}" title={m.label}>
  <span aria-hidden="true">{m.glyph}</span>
  {m.label}
</span>

<style>
  .risk {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 0.15rem 0.45rem;
    border-radius: 999px;
    border: 1px solid var(--color-border);
    white-space: nowrap;
  }
  .risk.passive {
    color: #8b949e;
  }
  .risk.mutating {
    color: #e3b341;
    border-color: #5c4a1f;
  }
  .risk.destructive {
    color: #ff7b72;
    border-color: #5c2b29;
  }
</style>
