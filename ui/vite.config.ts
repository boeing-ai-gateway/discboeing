import { readFileSync } from "node:fs";
import devtoolsJson from "vite-plugin-devtools-json";
import tailwindcss from "@tailwindcss/vite";
import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig, type Plugin } from "vite";

function normalizeModuleId(id: string): string {
	return id.replaceAll("\\", "/");
}

function patchNoVncBrowserCode(code: string): string {
	return code.replace(
		/= await _checkWebCodecsH264DecodeSupport\(\)/g,
		"= false",
	);
}

function isNoVncBrowserModule(id: string): boolean {
	const normalizedId = normalizeModuleId(id);
	return (
		normalizedId.includes("/@novnc/novnc/") &&
		normalizedId.endsWith("/browser.js")
	);
}

function fixNoVncCjs(): Plugin {
	return {
		name: "fix-novnc-cjs",
		enforce: "pre",
		load(id) {
			if (isNoVncBrowserModule(id)) {
				const code = readFileSync(id, "utf-8");
				return patchNoVncBrowserCode(code);
			}
		},
	};
}

function trackSSRBuild(): Plugin {
	let isSSRBuild = false;

	return {
		name: "track-ssr-build",
		configResolved(config) {
			isSSRBuild = Boolean(config.build.ssr);
		},
		config() {
			return {
				build: {
					rollupOptions: {
						output: {
							manualChunks(id: string) {
								const normalizedId = normalizeModuleId(id);

								if (isSSRBuild || !normalizedId.includes("node_modules")) {
									return;
								}

								// Isolate Monaco into its own chunk so Rollup can GC the rest
								// of the module graph before processing it. Monaco's source is
								// ~75 MB and includes 5 large worker bundles (including the full
								// TypeScript compiler), making it the primary driver of OOM
								// failures during CI builds.
								if (normalizedId.includes("/monaco-editor/")) {
									return "monaco-editor";
								}

								if (
									normalizedId.includes("/svelte/") ||
									normalizedId.includes("/@sveltejs/") ||
									normalizedId.includes("/bits-ui/") ||
									normalizedId.includes("/runed/") ||
									normalizedId.includes("/svelte-toolbelt/") ||
									normalizedId.includes("/paneforge/")
								) {
									return "svelte-core";
								}
							},
						},
					},
				},
			};
		},
	};
}

export default defineConfig(() => ({
	plugins: [
		trackSSRBuild(),
		fixNoVncCjs(),
		sveltekit(),
		tailwindcss(),
		devtoolsJson(),
	],
	test: {
		environment: "jsdom",
		include: ["src/lib/**/*.vitest.ts"],
	},
	server: { port: 3100, strictPort: true },
	preview: { port: 3100, strictPort: true },
	worker: {
		format: "es",
	},
	clearScreen: false,
	build: {
		sourcemap: true,
		// Increase chunk size warning limit for the Svelte UI bundle.
		chunkSizeWarningLimit: 4000,
		rollupOptions: {
			// Limit parallel file reads to reduce peak memory during the build.
			maxParallelFileOps: 20,
			onwarn(warning, defaultHandler) {
				defaultHandler(warning);
			},
		},
	},
	optimizeDeps: {
		include: ["@novnc/novnc/lib/rfb"],
		esbuildOptions: {
			plugins: [
				{
					name: "fix-novnc-cjs",
					setup(build) {
						build.onLoad(
							{ filter: /browser\.js$/, namespace: "file" },
							(args) => {
								if (!isNoVncBrowserModule(args.path)) return;
								const code = readFileSync(args.path, "utf-8");
								return {
									contents: patchNoVncBrowserCode(code),
									loader: "js",
								};
							},
						);
					},
				},
			],
		},
	},
}));
