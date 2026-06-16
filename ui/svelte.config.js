import adapter from "@sveltejs/adapter-static";
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

const webComponentPaths = ["/src/lib/web-components/markdown/"];

function isWebComponentFile(filename) {
	return webComponentPaths.some((path) => filename.includes(path));
}

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	onwarn(warning, defaultHandler) {
		if (
			warning.code === "options_missing_custom_element" &&
			warning.filename &&
			isWebComponentFile(warning.filename)
		) {
			return;
		}
		defaultHandler(warning);
	},
	kit: {
		adapter: adapter({
			fallback: "200.html",
		}),
		alias: {
			$components: "./src/lib/components",
			$utils: "./src/lib/utils",
		},
	},
	vitePlugin: {
		dynamicCompileOptions({ filename }) {
			if (isWebComponentFile(filename)) {
				return { customElement: true };
			}
		},
	},
};

export default config;
