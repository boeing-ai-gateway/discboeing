import esbuild from "esbuild";

const common = {
	bundle: true,
	format: "esm",
	platform: "browser",
	target: ["es2022"],
	sourcemap: true,
	logLevel: "info",
};

await esbuild.build({
	...common,
	entryPoints: ["./assets/js/app.ts"],
	splitting: true,
	outdir: "./static/assets",
	chunkNames: "chunks/[name]-[hash]",
	assetNames: "assets/[name]-[hash]",
	loader: {
		".woff": "file",
		".woff2": "file",
	},
});

await esbuild.build({
	...common,
	entryPoints: [{ in: "@pierre/diffs/worker/worker.js", out: "pierre-diff-worker" }],
	outfile: "./static/assets/pierre-diff-worker.js",
});
