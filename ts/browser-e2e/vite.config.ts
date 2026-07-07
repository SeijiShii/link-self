import { fileURLToPath } from "node:url";
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
      // Serve the linked @linkself/core sources AND its node_modules
      // (sqlite-wasm's sqlite3.wasm / OPFS async-proxy worker are fetched
      // at runtime from ts/linkself/node_modules via the file: link).
      // Root is app/, so ".." reaches ts/ — covers both packages.
      allow: [fileURLToPath(new URL("..", import.meta.url))],
    },
  },
  optimizeDeps: {
    // Per sqlite-wasm docs: keep it out of the prebundle so its worker
    // and wasm assets resolve correctly.
    exclude: ["@sqlite.org/sqlite-wasm"],
  },
});
