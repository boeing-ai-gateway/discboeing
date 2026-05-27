import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [svelte()],
	server: {
		port: 3334,
		proxy: {
			"/api": {
				target: "http://localhost:3333",
				changeOrigin: true,
				ws: true
			}
		}
	}
});
