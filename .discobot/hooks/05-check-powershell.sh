#!/bin/bash
#---
# name: PowerShell format and lint
# type: file
# pattern: "**/*.ps1"
#---
pnpm check:powershell:fix -- $DISCOBOT_CHANGED_FILES
