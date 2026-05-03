#!/bin/bash
#---
# name: Meta generate
# type: file
# pattern: "{meta/api/openapi.yaml,meta/api/types.schema.json}"
#---
cd meta && go generate ./...
