#!/bin/bash
#---
# name: Discobot Meta
# description: Central metadata, identity, authorization, and secret envelope service
# order: 3
# http: 3011
# path: /docs
#---

set -e

: "${META_PORT:=3011}"
: "${META_ADDR:=0.0.0.0:${META_PORT}}"
export META_ADDR

cd meta
exec go run github.com/air-verse/air@latest
