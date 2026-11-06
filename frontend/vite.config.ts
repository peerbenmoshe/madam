import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    // Fail rather than quietly move to another port.
    strictPort: true,
    // In `make dev` the page comes from Vite and the API from the Go server.
    // Passing /api/ through makes them one origin to the browser, as in
    // production, so no CORS is needed. 8080 is the server's default PORT.
    proxy: { "/api/": "http://127.0.0.1:8080" },
  },
});
