#!/bin/bash
#---
# name: OpenAPI generate
# type: file
# pattern: "server/{api/openapi.json,api/oapi-codegen.yaml,client/oapi-codegen.yaml}"
#---
set -euo pipefail

cd "${DISCBOEING_WORKSPACE:-$(pwd)}/server"
go generate ./api ./client
