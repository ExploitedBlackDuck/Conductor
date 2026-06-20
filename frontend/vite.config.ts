import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// The build output is embedded into the Go binary (frontend/dist → shell
// AssetServer). emptyOutDir keeps stale assets out of the embedded bundle.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
