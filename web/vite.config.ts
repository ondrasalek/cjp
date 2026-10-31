import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const apiTarget =
  (globalThis as { process?: { env?: Record<string, string | undefined> } })
    .process?.env?.VITE_API_TARGET ?? "http://localhost:8088";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/healthz": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/robots.txt": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/sitemap.xml": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/sitemaps": {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
});
