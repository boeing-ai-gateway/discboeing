import js from "@eslint/js";
import svelte from "eslint-plugin-svelte";
import globals from "globals";
import tseslint from "typescript-eslint";
import svelteConfig from "./svelte.config.js";

export default tseslint.config(
	{
		ignores: [
			".svelte-kit/**",
			".svelte-kit-hook/**",
			"build/**",
			"build-hook/**",
			"dist/**",
			"coverage/**",
		],
	},
	js.configs.recommended,
	...tseslint.configs.recommended,
	...svelte.configs.recommended,
	...svelte.configs.prettier,
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node,
			},
		},
	},
	{
		files: ["**/*.svelte", "**/*.svelte.js", "**/*.svelte.ts"],
		languageOptions: {
			parserOptions: {
				extraFileExtensions: [".svelte"],
				parser: tseslint.parser,
				svelteConfig,
			},
		},
	},
);
