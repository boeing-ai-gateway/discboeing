#!/bin/bash
#---
# name: Meta SQLite GUI
# description: datasette browser for the Discobot Meta database
# http: 8081
#---

set +x

DB="${HOME}/.local/share/discobot/meta.db"

if [ ! -f "$DB" ]; then
    echo "Database not found at: $DB"
    echo "Start the Meta service first to create the database."
    exit 1
fi

echo "Opening Meta SQLite GUI at http://localhost:8081"
echo "Database: $DB"

exec uvx --native-tls datasette serve "$DB" --port 8081 --host 0.0.0.0
