#!/bin/bash
#---
# name: ACP generate
# type: file
# pattern: "agent-go/acp/schema/schema.json"
#---
cd "${DISCOBOT_WORKSPACE:-$(pwd)}/agent-go" && go generate ./acp/protocol
