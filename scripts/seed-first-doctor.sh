#!/usr/bin/env bash
# seed-first-doctor.sh
# Creates the first DOCTOR account directly in PostgreSQL.
# Run this ONCE after provisioning the user-service for the first time.
#
# Usage:
#   ./seed-first-doctor.sh \
#     --first-name "Diego" \
#     --last-name "Murcia" \
#     --email "doctor@example.com" \
#     --password "MyPassword123"
set -euo pipefail

KV="kv-sentinel-he"
RG="rg-sentinel-health-engine"
PG_SERVER="pg-sentinel-he"
PG_DB="sentinel_users"

FIRST_NAME=""
LAST_NAME=""
EMAIL=""
PASSWORD=""

# Parse named arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --first-name) FIRST_NAME="$2"; shift 2 ;;
    --last-name)  LAST_NAME="$2";  shift 2 ;;
    --email)      EMAIL="$2";      shift 2 ;;
    --password)   PASSWORD="$2";   shift 2 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done

if [[ -z "$FIRST_NAME" || -z "$LAST_NAME" || -z "$EMAIL" || -z "$PASSWORD" ]]; then
  echo "Usage: $0 --first-name NAME --last-name LASTNAME --email EMAIL --password PASSWORD"
  exit 1
fi

# ── Generate bcrypt hash using Docker (avoids local Go/Python dependency) ────
echo ">>> Generating bcrypt hash for password..."
PASSWORD_HASH=$(docker run --rm golang:1.22-alpine sh -c "
  apk add -q --no-cache go 2>/dev/null; \
  cat > /tmp/hash.go << 'GOEOF'
package main
import (
  \"fmt\"
  \"golang.org/x/crypto/bcrypt\"
  \"os\"
)
func main() {
  hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
  if err != nil { panic(err) }
  fmt.Print(string(hash))
}
GOEOF
  cd /tmp && go mod init hash && go get golang.org/x/crypto && go run hash.go '$PASSWORD'
" 2>/dev/null)

if [[ -z "$PASSWORD_HASH" ]]; then
  echo "ERROR: failed to generate bcrypt hash. Is Docker running?"
  exit 1
fi

# ── Get PostgreSQL credentials from Key Vault ─────────────────────────────────
echo ">>> Reading DATABASE_URL from Key Vault..."
DATABASE_URL=$(az keyvault secret show --vault-name "$KV" --name "postgres-database-url" --query value -o tsv)

# Extract components from DATABASE_URL (format: host=... port=... dbname=... user=... password=... sslmode=...)
PG_HOST=$(echo "$DATABASE_URL" | grep -oP 'host=\K[^\s]+')
PG_PORT=$(echo "$DATABASE_URL" | grep -oP 'port=\K[^\s]+')
PG_USER=$(echo "$DATABASE_URL" | grep -oP 'user=\K[^\s]+')
PG_PASS=$(echo "$DATABASE_URL" | grep -oP 'password=\K[^\s]+')

# ── Add firewall rule for current IP (temporary) ──────────────────────────────
CURRENT_IP=$(curl -s https://api.ipify.org)
echo ">>> Adding firewall rule for your IP: $CURRENT_IP..."
az postgres flexible-server firewall-rule create \
  --resource-group "$RG" \
  --name "$PG_SERVER" \
  --rule-name "TempSeed" \
  --start-ip-address "$CURRENT_IP" \
  --end-ip-address "$CURRENT_IP" \
  --output none

# ── Run the INSERT ─────────────────────────────────────────────────────────────
echo ">>> Inserting doctor account..."
DOCTOR_ID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())")

PGPASSWORD="$PG_PASS" psql \
  "host=${PG_HOST} port=${PG_PORT} dbname=${PG_DB} user=${PG_USER} sslmode=require" \
  -c "INSERT INTO users (id, email, password_hash, role, first_name, last_name, is_active)
      VALUES ('${DOCTOR_ID}', '${EMAIL}', '${PASSWORD_HASH}', 'DOCTOR', '${FIRST_NAME}', '${LAST_NAME}', true)
      ON CONFLICT (email) DO NOTHING;"

# ── Remove temporary firewall rule ─────────────────────────────────────────────
echo ">>> Removing temporary firewall rule..."
az postgres flexible-server firewall-rule delete \
  --resource-group "$RG" \
  --name "$PG_SERVER" \
  --rule-name "TempSeed" \
  --yes \
  --output none

echo ""
echo "✅ Doctor account created."
echo ""
echo "  ID        : $DOCTOR_ID"
echo "  Name      : $FIRST_NAME $LAST_NAME"
echo "  Email     : $EMAIL"
echo "  Role      : DOCTOR"
echo ""
echo "Login test:"
echo "  USER_SERVICE_URL=\$(az containerapp show --name user-service --resource-group $RG --query properties.configuration.ingress.fqdn -o tsv)"
echo "  curl -s -X POST https://\$USER_SERVICE_URL/v1/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}'"
