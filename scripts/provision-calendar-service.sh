#!/usr/bin/env bash
# provision-calendar-service.sh
# Creates the sentinel_calendar database on the existing PostgreSQL Flexible Server.
# Run ONCE. Requires provision-user-service.sh to have run first (server already exists).
set -euo pipefail

KV="kv-sentinel-he"
RG="rg-sentinel-health-engine"
PG_SERVER="pg-sentinel-he"

echo ">>> Reading credentials from Key Vault..."
DATABASE_URL=$(az keyvault secret show --vault-name "$KV" --name "postgres-database-url" --query value -o tsv)

# Extract host/user/pass from DSN (format: host=X port=5432 dbname=Y user=Z password=W sslmode=require)
PG_HOST=$(echo "$DATABASE_URL" | grep -o 'host=[^ ]*' | cut -d= -f2)
PG_USER=$(echo "$DATABASE_URL" | grep -o 'user=[^ ]*' | cut -d= -f2)
PG_PASS=$(echo "$DATABASE_URL" | grep -o 'password=[^ ]*' | cut -d= -f2)
PG_PORT=$(echo "$DATABASE_URL" | grep -o 'port=[^ ]*' | cut -d= -f2)

echo ">>> Adding temporary firewall rule for current IP..."
CURRENT_IP=$(curl -s https://api.ipify.org)
az postgres flexible-server firewall-rule create \
  --resource-group "$RG" --name "$PG_SERVER" \
  --rule-name "TempCalendar" \
  --start-ip-address "$CURRENT_IP" --end-ip-address "$CURRENT_IP" \
  --output none

echo ">>> Creating sentinel_calendar database..."
PGPASSWORD="$PG_PASS" psql \
  "host=${PG_HOST} port=${PG_PORT} dbname=postgres user=${PG_USER} sslmode=require" \
  -c "CREATE DATABASE sentinel_calendar;" 2>/dev/null || echo "Database may already exist, continuing..."

CALENDAR_URL="host=${PG_HOST} port=${PG_PORT} dbname=sentinel_calendar user=${PG_USER} password=${PG_PASS} sslmode=require"

echo ">>> Storing calendar DATABASE_URL in Key Vault..."
az keyvault secret set --vault-name "$KV" --name "calendar-database-url" --value "$CALENDAR_URL"

echo ">>> Removing temporary firewall rule..."
az postgres flexible-server firewall-rule delete \
  --resource-group "$RG" --name "$PG_SERVER" --rule-name "TempCalendar" --yes --output none

echo ""
echo "✅ Calendar database provisioned."
echo "Secret stored: calendar-database-url"
echo "Next step: bash scripts/deploy-all.sh"
