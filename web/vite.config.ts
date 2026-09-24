import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

// Port 5173 is the origin registered for the IAM "cloud web (dev)" client.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // stroppy-server API + WS; prod serves the SPA from the same origin.
      "/api": { target: "http://localhost:18347", changeOrigin: true, ws: true },
      // IAM, reverse-proxied by stroppy-server so auth stays same-origin.
      "/v1": { target: "http://localhost:18347", changeOrigin: true },
      // embedded Grafana relay.
      "/grafana": { target: "http://localhost:18347", changeOrigin: true },
    },
  },
});
