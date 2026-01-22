#!/bin/bash
set -e

# Wait for ClickHouse to be ready
until clickhouse-client --host localhost --query "SELECT 1" > /dev/null 2>&1; do
    echo "Waiting for ClickHouse to start..."
    sleep 1
done

echo "ClickHouse is ready. Initializing schema..."

# Create database
clickhouse-client --host localhost --query "CREATE DATABASE IF NOT EXISTS terracost"

# Run schema migrations
for f in /docker-entrypoint-initdb.d/*.sql; do
    if [ -f "$f" ]; then
        echo "Running $f..."
        clickhouse-client --host localhost --database terracost --multiquery < "$f"
    fi
done

echo "Schema initialization complete."
