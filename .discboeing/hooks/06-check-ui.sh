#!/bin/bash
#---
# name: Svelte UI check
# type: file
# pattern: "ui/**/*.{ts,tsx,js,jsx,svelte,json}"
#---
scripts/temproot.sh pnpm ui:typecheck
