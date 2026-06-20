<script lang="ts">
  // P0 foundation screen. This is intentionally minimal: it proves the desktop
  // shell opens, mounts the Svelte app, and renders. The navigation rail and
  // feature views (Transfers, Remotes, Mounts, Bisync, History, Audit — §7.11)
  // are built in later phases against real bindings, not faked here.
  const buildLabel = "Phase 0 — foundation";
</script>

<main class="screen">
  <section class="card">
    <h1 class="title">Conductor</h1>
    <p class="subtitle">A desktop control panel for rclone.</p>
    <p class="phase">{buildLabel}</p>
  </section>
</main>

<style>
  .screen {
    display: grid;
    place-items: center;
    height: 100%;
    padding: var(--space-6);
  }

  .card {
    text-align: center;
    padding: var(--space-6);
    border: 1px solid var(--color-border);
    border-radius: 12px;
    background: var(--color-surface);
    min-width: 22rem;
  }

  .title {
    margin: 0;
    font-size: 2rem;
    letter-spacing: -0.02em;
  }

  .subtitle {
    margin: var(--space-2) 0 var(--space-4);
    color: var(--color-text-muted);
  }

  .phase {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    color: var(--color-accent);
  }
</style>
