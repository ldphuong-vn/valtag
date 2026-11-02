#!/bin/bash
# Restore Valt database from backup
# Usage: ./scripts/restore-backup.sh <backup-file>

set -e
if [ -z "$1" ]; then
    echo "Usage: $0 <backup-file.sql.gz>"
    echo "Available backups:"
    ls -la /opt/valtag/backups/
    exit 1
fi

echo "WARNING: This will DROP and recreate the database."
read -p "Continue? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then exit 1; fi

BACKUP=$1
echo "Restoring from $BACKUP..."
gunzip -c "$BACKUP" | docker exec -i valtag-postgres-1 psql -U valtag -d valtag
echo "Restore complete."
