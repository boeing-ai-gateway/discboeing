#!/bin/bash
#---
# name: Install dependencies
# type: session
#---
# Install/link Node.js dependencies without lifecycle scripts first so
# Rustywind is present in node_modules before its downloader runs.
pnpm install --frozen-lockfile --ignore-scripts 2>&1 || pnpm install --ignore-scripts 2>&1

# Install Rustywind's native binary without the sandbox proxy. Its npm
# postinstall downloader is not compatible with the proxy agent used in
# Discobot sessions.
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u http_proxy -u https_proxy -u all_proxy \
	pnpm rebuild rustywind 2>&1

# Install Node.js dependencies (needed for biome, tsc, and other tools)
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1

# Download Go module dependencies
cd server && go mod download 2>&1 &
cd proxy && go mod download 2>&1 &
cd agent && go mod download 2>&1 &
wait
