import { defineConfig } from "vite";

export default defineConfig({
  publicDir: "fixtures",
  build: {
    chunkSizeWarningLimit: 600
  },
  server: {
    host: "0.0.0.0",
    port: 8090,
    strictPort: true
  },
  test: {
    environment: "jsdom"
  }
});
