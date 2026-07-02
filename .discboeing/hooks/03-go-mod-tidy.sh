#!/bin/bash
#---
# name: Go mod tidy
# type: file
# pattern: "**/go.mod"
#---
for f in $DISCBOEING_CHANGED_FILES; do
	if [ ! -f "$f" ]; then
		continue
	fi
	dir=$(dirname "$f")
	(cd "$dir" && go mod tidy 2>&1)
done
