import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

const outDir = process.env.SVELTEKIT_OUTDIR || ".svelte-kit";

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		outDir,
		adapter: adapter({
			fallback: "200.html",
		}),
		alias: {
			$components: "./src/lib/components",
			$utils: "./src/lib/utils",
		},
	},
};

export default config;
