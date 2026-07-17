import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: Object.fromEntries(
      ["/api", "/downloads", "/healthz", "/install.sh", "/install.ps1"]
        .map((path) => [path, "http://127.0.0.1:8080"]),
    ),
  },
});
