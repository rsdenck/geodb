#!/bin/bash
# Restore database from compressed dump
# Usage: ./db/restore.sh [dump_file]

DUMP="${1:-db/geoip.dump}"

if [ ! -f "$DUMP" ]; then
    echo "Dump file not found: $DUMP"
    echo "Usage: $0 [path/to/dump]"
    exit 1
fi

echo "Restoring database from $DUMP ..."
echo "Make sure database 'geoip' exists and is empty."

PGPASSWORD=geoip123 pg_restore -h 127.0.0.1 -U geoip -d geoip --no-owner --no-privileges --disable-triggers "$DUMP"

if [ $? -eq 0 ]; then
    echo "Restore complete."
else
    echo "Restore failed or had warnings (circular FK constraints expected)."
fi
