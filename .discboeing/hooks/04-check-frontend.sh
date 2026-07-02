#!/bin/bash
#---
# name: Frontend check
# type: file
# pattern: "**/*.{ts,tsx,js,jsx,svelte,json}"
#---
pnpm ui:format
pnpm --dir ./ui lint:fix
