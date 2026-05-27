#!/bin/bash
#---
# name: Discobot
# description: Go + templ Datastar app shell
# order: 2
# http: 3300
# path: /
#---

set -e

exec pnpm discobot:dev
