#!/bin/bash
#---
# name: Svelte UI build
# type: file
# pattern: "ui/**/*.{ts,tsx,js,jsx,svelte,json}"
#---
SVELTEKIT_OUTDIR=.svelte-kit-hook pnpm ui:build
