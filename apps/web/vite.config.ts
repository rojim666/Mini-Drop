import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: process.env.MINIDROP_API_BASE_URL ?? `http://127.0.0.1:${process.env.MINIDROP_API_PORT ?? "8080"}`,
        changeOrigin: true,
      },
      "/healthz": {
        target: process.env.MINIDROP_API_BASE_URL ?? `http://127.0.0.1:${process.env.MINIDROP_API_PORT ?? "8080"}`,
        changeOrigin: true,
      },
    },
  },
});
