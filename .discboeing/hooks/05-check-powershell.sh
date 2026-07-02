#!/bin/bash
#---
# name: PowerShell format and lint
# type: file
# pattern: "**/*.ps1"
#---
# shellcheck disable=SC2086
pnpm check:powershell:fix -- $DISCBOEING_CHANGED_FILES
