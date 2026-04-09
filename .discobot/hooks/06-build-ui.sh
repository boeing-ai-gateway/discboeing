#!/bin/bash
#---
# name: Svelte UI build
# type: file
# pattern: "ui/**/*.{ts,tsx,js,jsx,svelte,json}"
#---
rm -rf ui/.svelte-kit-hook ui/build-hook
SVELTEKIT_OUTDIR=.svelte-kit-hook SVELTEKIT_BUILD_DIR=build-hook pnpm ui:build
