#!/bin/bash
#---
# name: Svelte UI check
# type: file
# pattern: "ui/**/*.{ts,tsx,js,jsx,svelte,json}"
#---
cd ui && pnpx sv check
