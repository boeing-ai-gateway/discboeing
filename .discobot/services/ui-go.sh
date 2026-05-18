#!/bin/bash
#---
# name: Discobot UI Go
# description: Go + templ UI development server
# order: 1
# http: 3200
# path: /
#---

set -e

exec pnpm ui-go:dev
