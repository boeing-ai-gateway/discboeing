import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

const outDir = process.env.SVELTEKIT_OUTDIR || ".svelte-kit";
const buildDir = process.env.SVELTEKIT_BUILD_DIR || "build";

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		outDir,
		adapter: adapter({
			pages: buildDir,
			assets: buildDir,
			fallback: "200.html",
		}),
		alias: {
			$components: "./src/lib/components",
			$utils: "./src/lib/utils",
		},
	},
};

export default config;
