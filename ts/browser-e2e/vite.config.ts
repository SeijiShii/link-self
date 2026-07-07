import { defineConfig } from "vite";

export default defineConfig({
  root: "app",
  server: {
    port: 5199,
    strictPort: true,
    headers: {
      // sqlite-wasm's OPFS VFS uses SharedArrayBuffer + Atomics, which
      // require cross-origin isolation.
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "require-corp",
    },
    fs: {
      // allow serving the linked @linkself/core TS sources
      allow: [".."],
    },
  },
  optimizeDeps: {
    // Per sqlite-wasm docs: keep it out of the prebundle so its worker
    // and wasm assets resolve correctly.
    exclude: ["@sqlite.org/sqlite-wasm"],
  },
});
