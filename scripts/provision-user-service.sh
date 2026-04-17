#!/usr/bin/env bash
# provision-user-service.sh
# Provisions Azure PostgreSQL Flexible Server and stores secrets in Key Vault.
# Run ONCE to set up infrastructure for user-service.
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
LOCATION="centralus"

# ── Configurable ─────────────────────────────────────────────────────────────
PG_SERVER_NAME="pg-sentinel-he"
PG_ADMIN_USER="sentineladmin"
PG_DB_NAME="sentinel_users"
PG_SKU="Standard_B1ms"   # cheapest tier, sufficient for MVP
PG_TIER="Burstable"
PG_VERSION="16"

echo ">>> Generating PostgreSQL admin password..."
PG_ADMIN_PASSWORD=$(openssl rand -base64 24 | tr -d '+=/' | head -c 32)

echo ">>> Creating Azure PostgreSQL Flexible Server: $PG_SERVER_NAME..."
az postgres flexible-server create \
  --resource-group "$RG" \
  --name "$PG_SERVER_NAME" \
  --location "$LOCATION" \
  --admin-user "$PG_ADMIN_USER" \
  --admin-password "$PG_ADMIN_PASSWORD" \
  --sku-name "$PG_SKU" \
  --tier "$PG_TIER" \
  --version "$PG_VERSION" \
  --storage-size 32 \
  --public-access 0.0.0.0 \
  --yes

echo ">>> Creating database: $PG_DB_NAME..."
az postgres flexible-server db create \
  --resource-group "$RG" \
  --server-name "$PG_SERVER_NAME" \
  --database-name "$PG_DB_NAME"

echo ">>> Allowing Azure Container Apps environment to access PostgreSQL..."
# Allow all Azure services (simplified for MVP; tighten with VNet integration in production)
az postgres flexible-server firewall-rule create \
  --resource-group "$RG" \
  --name "$PG_SERVER_NAME" \
  --rule-name "AllowAzureServices" \
  --start-ip-address 0.0.0.0 \
  --end-ip-address 0.0.0.0

echo ">>> Building DATABASE_URL..."
PG_HOST="${PG_SERVER_NAME}.postgres.database.azure.com"
DATABASE_URL="host=${PG_HOST} port=5432 dbname=${PG_DB_NAME} user=${PG_ADMIN_USER} password=${PG_ADMIN_PASSWORD} sslmode=require"

echo ">>> Generating INTERNAL_API_KEY and JWT_SECRET..."
INTERNAL_API_KEY=$(openssl rand -hex 32)
JWT_SECRET=$(openssl rand -hex 32)

echo ">>> Storing secrets in Key Vault: $KV..."
az keyvault secret set --vault-name "$KV" --name "postgres-database-url"  --value "$DATABASE_URL"
az keyvault secret set --vault-name "$KV" --name "internal-api-key"        --value "$INTERNAL_API_KEY"
az keyvault secret set --vault-name "$KV" --name "jwt-secret"              --value "$JWT_SECRET"

echo ""
echo "✅ Provisioning complete."
echo ""
echo "PostgreSQL host  : $PG_HOST"
echo "Database         : $PG_DB_NAME"
echo "Admin user       : $PG_ADMIN_USER"
echo ""
echo "Secrets stored in Key Vault '$KV':"
echo "  postgres-database-url"
echo "  internal-api-key"
echo "  jwt-secret"
echo ""
echo "Next step: run deploy-user-service.sh"
