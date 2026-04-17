# Sentinel Health Engine — Deployment Guide

This guide covers deploying the full Sentinel Health Engine system on Azure Container Apps,
with particular focus on the **user-service** (the most recently added microservice).
The other three services (telemetry-service, health-rules-service, alerts-service) are
assumed to already be deployed and operational.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Deploying user-service for the First Time](#2-deploying-user-service-for-the-first-time)
3. [Seeding the First Doctor Account](#3-seeding-the-first-doctor-account)
4. [Verifying the Deployment](#4-verifying-the-deployment)
5. [Updating an Existing Service After Code Changes](#5-updating-an-existing-service-after-code-changes)
6. [Environment Variable Reference](#6-environment-variable-reference)
7. [Powering Services On and Off](#7-powering-services-on-and-off)

---

## 1. Prerequisites

### Required Tools

| Tool | Minimum Version | Notes |
|------|----------------|-------|
| Azure CLI (`az`) | 2.55+ | `az --version` |
| Docker Desktop | 4.x | Must be running |
| Git Bash (Windows) | Any | Scripts use `bash` shebang |
| `openssl` | Any | Used by provision script for secret generation |

Install the Azure CLI extensions if not already present:

```bash
az extension add --name containerapp
az extension add --name rdbms-connect   # optional, useful for DB debugging
```

### Azure Permissions

Your account or service principal must hold at least the following roles on
the `rg-sentinel-health-engine` resource group:

- **Contributor** — to create and update Container Apps, PostgreSQL, and firewall rules
- **Key Vault Secrets Officer** — to read and write Key Vault secrets

Verify your current login:

```bash
az account show
az role assignment list --assignee $(az account show --query user.name -o tsv) \
  --resource-group rg-sentinel-health-engine --output table
```

### Repository Layout

All scripts and commands in this guide assume you are running from the **project root**:

```
sentinel-health-engine/
├── scripts/
│   ├── provision-user-service.sh
│   ├── deploy-user-service.sh
│   ├── update-telemetry-service.sh
│   ├── update-alerts-with-userservice.sh
│   ├── powerup-services.sh
│   └── shutdown-services.sh
└── services/
    ├── user-service/
    ├── telemetry-service/
    ├── health-rules-service/
    └── alerts-service/
```

Always `cd` to the project root before running any script:

```bash
cd /path/to/sentinel-health-engine
```

---

## 2. Deploying user-service for the First Time

This section applies when the three original services are already running and you are
adding user-service to the existing environment. Follow all four steps in order.

### Step 2a — Provision PostgreSQL and Store Secrets

The provision script creates the Azure PostgreSQL Flexible Server, the application
database, a firewall rule to allow Azure Container Apps access, and stores all generated
secrets in Key Vault. **Run this script only once.**

```bash
bash scripts/provision-user-service.sh
```

What this script does:

- Creates `pg-sentinel-he` (Standard_B1ms, Burstable tier, PostgreSQL 16, 32 GB storage)
- Creates the `sentinel_users` database inside that server
- Adds a firewall rule `AllowAzureServices` (IP range 0.0.0.0–0.0.0.0)
- Generates a random admin password, JWT secret, and internal API key using `openssl`
- Stores three secrets in `kv-sentinel-he`:
  - `postgres-database-url` — full libpq connection string
  - `jwt-secret` — used for signing user JWTs
  - `internal-api-key` — shared key for service-to-service calls

Expected output (abbreviated):

```
>>> Generating PostgreSQL admin password...
>>> Creating Azure PostgreSQL Flexible Server: pg-sentinel-he...
>>> Creating database: sentinel_users...
>>> Allowing Azure Container Apps environment to access PostgreSQL...
>>> Generating INTERNAL_API_KEY and JWT_SECRET...
>>> Storing secrets in Key Vault: kv-sentinel-he...

✅ Provisioning complete.

PostgreSQL host  : pg-sentinel-he.postgres.database.azure.com
Database         : sentinel_users
Admin user       : sentineladmin
```

> **Note:** The PostgreSQL admin password is generated dynamically and stored
> exclusively in Key Vault. It is never printed to the terminal after the
> `provision` run completes. If you need it later, retrieve it from Key Vault:
>
> ```bash
> az keyvault secret show --vault-name kv-sentinel-he \
>   --name postgres-database-url --query value -o tsv
> ```

### Step 2b — Build and Deploy user-service

The deploy script reads the secrets just stored, builds the Docker image, pushes it
to ACR, and creates (or updates) the `user-service` Container App.

```bash
bash scripts/deploy-user-service.sh
```

What this script does:

1. Logs in to `crsentinelhe.azurecr.io`
2. Builds `services/user-service/Dockerfile` (build context is the project root)
3. Pushes the image tagged with the current timestamp (`YYYYMMDDHHMMSS`)
4. Reads all required secrets from Key Vault
5. Creates the Container App with external ingress on port 8080
   (or updates it if it already exists)
6. Prints the public FQDN of user-service
7. Automatically calls `update-telemetry-service.sh` and
   `update-alerts-with-userservice.sh` with the new URL

After success the script prints:

```
✅ user-service deployed.
URL: https://user-service.<hash>.eastus.azurecontainerapps.io
```

Save this URL — you will use it for all curl commands in this guide.

### Step 2c — Update telemetry-service

> **This step is performed automatically by `deploy-user-service.sh`** when user-service
> is deployed for the first time. Run it manually only if you need to re-point
> telemetry-service to a new user-service URL without redeploying user-service.

```bash
bash scripts/update-telemetry-service.sh https://<user-service-fqdn>
```

This replaces the old `AUTHORIZED_DEVICES` static map with two new environment
variables: `USER_SERVICE_URL` and `USER_SERVICE_API_KEY`. From this point,
telemetry-service will validate incoming IoT device identifiers by calling
`GET /v1/internal/devices/:identifier` on user-service.

### Step 2d — Update alerts-service

> Same as above — called automatically by `deploy-user-service.sh`. Run manually
> only when needed.

```bash
bash scripts/update-alerts-with-userservice.sh https://<user-service-fqdn>
```

This replaces the old `PATIENT_CONTACTS` static JSON map with `USER_SERVICE_URL`
and `USER_SERVICE_API_KEY`. From this point, alerts-service resolves push/email
recipients by calling `GET /v1/internal/patients/:id/contacts` on user-service.

---

## 3. Seeding the First Doctor Account

user-service has no registration endpoint and no web admin panel. All user creation
goes through the `POST /v1/patients` API endpoint, which is gated behind the `DOCTOR`
role. This creates a chicken-and-egg problem for the very first account.

The solution is to insert the first doctor directly into PostgreSQL.

### 3a — Generate a bcrypt Password Hash

You need a bcrypt-hashed password. Use one of the following methods:

**Using Docker (recommended — no extra tools needed):**

```bash
docker run --rm alpine/openssl sh -c \
  "apk add --no-cache python3 py3-bcrypt 2>/dev/null; \
   python3 -c \"import bcrypt; print(bcrypt.hashpw(b'YourPassword123!', bcrypt.gensalt(12)).decode())\""
```

**Using Go (if Go is installed):**

```bash
go run -e 'package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    h, _ := bcrypt.GenerateFromPassword([]byte("YourPassword123!"), 12)
    fmt.Println(string(h))
}'
```

**Using an online tool (for development only):**

Visit [bcrypt.online](https://bcrypt.online) with cost factor 12.

The hash will look like:
```
$2a$12$abcdefghijklmnopqrstuvuABCDEFGHIJKLMNOPQRSTUVWXYZ01234
```

### 3b — Connect to PostgreSQL

Retrieve the connection details from Key Vault:

```bash
az keyvault secret show --vault-name kv-sentinel-he \
  --name postgres-database-url --query value -o tsv
```

Connect using the Azure CLI (requires `rdbms-connect` extension and your client IP
whitelisted, or use Azure Cloud Shell):

```bash
az postgres flexible-server connect \
  --name pg-sentinel-he \
  --username sentineladmin \
  --database-name sentinel_users
```

Or connect with `psql` directly:

```bash
psql "host=pg-sentinel-he.postgres.database.azure.com port=5432 \
      dbname=sentinel_users user=sentineladmin sslmode=require"
```

> **Firewall note:** The provision script only whitelists Azure services. To connect
> from your local machine, add a firewall rule for your IP:
>
> ```bash
> MY_IP=$(curl -s https://api.ipify.org)
> az postgres flexible-server firewall-rule create \
>   --resource-group rg-sentinel-health-engine \
>   --name pg-sentinel-he \
>   --rule-name "AllowMyIP" \
>   --start-ip-address "$MY_IP" \
>   --end-ip-address "$MY_IP"
> ```

### 3c — Insert the Doctor Record

Run the following SQL, replacing the placeholder values:

```sql
INSERT INTO users (
    id,
    email,
    password_hash,
    role,
    first_name,
    last_name,
    is_active,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'doctor@hospital.com',
    '$2a$12$REPLACE_WITH_ACTUAL_BCRYPT_HASH',
    'DOCTOR',
    'Jane',
    'Smith',
    true,
    NOW(),
    NOW()
);
```

Verify the insert:

```sql
SELECT id, email, role, first_name, last_name, created_at FROM users WHERE role = 'DOCTOR';
```

### 3d — Verify Login

```bash
USER_SERVICE_URL="https://<your-user-service-fqdn>"

curl -s -X POST "$USER_SERVICE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@hospital.com","password":"YourPassword123!"}' | jq .
```

A successful response:

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 900
}
```

Store the access token for subsequent requests:

```bash
DOCTOR_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## 4. Verifying the Deployment

### Health Check Endpoints

Each service exposes `GET /health`. Use the Container App FQDNs:

```bash
# Retrieve FQDNs
for SVC in user-service telemetry-service health-rules-service alerts-service; do
  FQDN=$(az containerapp show \
    --name "$SVC" \
    --resource-group rg-sentinel-health-engine \
    --query properties.configuration.ingress.fqdn -o tsv)
  echo "$SVC → https://$FQDN/health"
done
```

Then check each one:

```bash
curl -s https://<user-service-fqdn>/health | jq .
# {"status":"ok","service":"user-service"}

curl -s https://<telemetry-service-fqdn>/health | jq .
# {"status":"ok","service":"telemetry-service"}

curl -s https://<health-rules-service-fqdn>/health | jq .
# {"status":"ok","service":"health-rules-service"}

curl -s https://<alerts-service-fqdn>/health | jq .
# {"status":"ok","service":"alerts-service"}
```

### Verify user-service API

```bash
# Login as doctor
curl -s -X POST "$USER_SERVICE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@hospital.com","password":"YourPassword123!"}' | jq .

# Get own profile (with token from above)
curl -s "$USER_SERVICE_URL/v1/users/me" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" | jq .
```

### Verify Internal Endpoints (Service-to-Service)

Retrieve the internal API key from Key Vault:

```bash
INTERNAL_API_KEY=$(az keyvault secret show \
  --vault-name kv-sentinel-he \
  --name internal-api-key --query value -o tsv)
```

```bash
# Check that telemetry-service can validate a device
curl -s "$USER_SERVICE_URL/v1/internal/devices/mobile-gateway-01" \
  -H "X-Internal-API-Key: $INTERNAL_API_KEY" | jq .

# Check that alerts-service can get contacts for a patient
curl -s "$USER_SERVICE_URL/v1/internal/patients/<patient-id>/contacts" \
  -H "X-Internal-API-Key: $INTERNAL_API_KEY" | jq .
```

### Verify Container App Replica Status

```bash
az containerapp replica list \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --output table
```

All four services should show at least one running replica.

---

## 5. Updating an Existing Service After Code Changes

The workflow for any code change is: build → push → update the Container App.

### Step-by-Step

```bash
# 1. Set variables
SERVICE="user-service"     # or telemetry-service, health-rules-service, alerts-service
ACR="crsentinelhe"
RG="rg-sentinel-health-engine"
TAG=$(date +%Y%m%d%H%M%S)
IMAGE="${ACR}.azurecr.io/${SERVICE}:${TAG}"

# 2. Log in to ACR
az acr login --name "$ACR"

# 3. Build (run from project root; Dockerfile path depends on the service)
docker build --no-cache \
  -t "$IMAGE" \
  -f "services/${SERVICE}/Dockerfile" \
  .

# 4. Push
docker push "$IMAGE"

# 5. Update the Container App with the new image
az containerapp update \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --image "$IMAGE"
```

### Watching the Rollout

```bash
az containerapp revision list \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --output table
```

The most recent revision will show as `Active` once traffic is shifted to it.

### Rolling Back

```bash
# List revisions
az containerapp revision list \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --query "[].{name:name,active:properties.active,created:properties.createdTime}" \
  --output table

# Activate a previous revision
az containerapp revision activate \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --revision <previous-revision-name>
```

---

## 6. Environment Variable Reference

### user-service

| Variable | Source | Description |
|----------|--------|-------------|
| `DATABASE_URL` | Key Vault: `postgres-database-url` | libpq connection string for PostgreSQL |
| `JWT_SECRET` | Key Vault: `jwt-secret` | HMAC key for signing access and refresh tokens |
| `INTERNAL_API_KEY` | Key Vault: `internal-api-key` | Key for internal service-to-service endpoints |
| `ACS_CONNECTION_STRING` | Key Vault: `acs-connection-string` | Azure Communication Services connection string |
| `ACS_SENDER_ADDRESS` | Key Vault: `acs-sender-address` | Verified sender email for ACS |
| `LOG_LEVEL` | Hardcoded in script | `info` (use `debug` for troubleshooting) |

### telemetry-service

| Variable | Source | Description |
|----------|--------|-------------|
| `IOTHUB_EVENTHUB_CONNECTION_STRING` | Key Vault: `iothub-eventhub-connection-string` | IoT Hub Event Hub-compatible endpoint |
| `IOTHUB_EVENTHUB_NAME` | Key Vault: `iothub-eventhub-name` | Event Hub name |
| `IOTHUB_CONSUMER_GROUP` | Hardcoded | `telemetry-service` |
| `AZURE_STORAGE_CONNECTION_STRING` | Key Vault: `storage-connection-string` | For checkpoint storage |
| `CHECKPOINT_CONTAINER_NAME` | Hardcoded | `iothub-checkpoints` |
| `SERVICE_BUS_CONNECTION_STRING` | Key Vault: `servicebus-connection-string` | For publishing telemetry events |
| `TELEMETRY_TOPIC_NAME` | Hardcoded | `telemetry-received` |
| `COSMOS_ENDPOINT` | Key Vault: `cosmos-endpoint` | Cosmos DB endpoint URL |
| `COSMOS_KEY` | Key Vault: `cosmos-key` | Cosmos DB primary key |
| `COSMOS_DATABASE` | Hardcoded | `sentinel-health` |
| `COSMOS_CONTAINER` | Hardcoded | `telemetry` |
| `USER_SERVICE_URL` | Injected at deploy time | FQDN of user-service |
| `USER_SERVICE_API_KEY` | Key Vault: `internal-api-key` | Shared key for internal calls |
| `LOG_LEVEL` | Hardcoded | `info` |

### health-rules-service

| Variable | Description |
|----------|-------------|
| `SERVICE_BUS_CONNECTION_STRING` | For consuming `telemetry-received` and publishing `anomaly-detected` |
| `TELEMETRY_TOPIC_NAME` | `telemetry-received` |
| `TELEMETRY_SUBSCRIPTION_NAME` | `health-rules-service` |
| `ANOMALY_TOPIC_NAME` | `anomaly-detected` |
| `LOG_LEVEL` | `info` |

> health-rules-service does not call user-service and does not need `USER_SERVICE_URL`.

### alerts-service

| Variable | Source | Description |
|----------|--------|-------------|
| `SERVICE_BUS_CONNECTION_STRING` | Key Vault: `servicebus-connection-string` | For consuming `anomaly-detected` |
| `ANOMALY_TOPIC_NAME` | Hardcoded | `anomaly-detected` |
| `ANOMALY_SUBSCRIPTION_NAME` | Hardcoded | `alerts-service` |
| `COSMOS_ENDPOINT` | Key Vault: `cosmos-endpoint` | Cosmos DB endpoint URL |
| `COSMOS_KEY` | Key Vault: `cosmos-key` | Cosmos DB primary key |
| `COSMOS_DATABASE` | Hardcoded | `sentinel-health` |
| `COSMOS_ALERTS_CONTAINER` | Hardcoded | `alerts` |
| `ACS_CONNECTION_STRING` | Key Vault: `acs-connection-string` | Azure Communication Services |
| `ACS_SENDER_ADDRESS` | Hardcoded in script | Verified ACS sender email |
| `FIREBASE_CREDENTIALS_JSON` | Key Vault: `firebase-credentials-json` | Firebase Admin SDK credentials (JSON) |
| `USER_SERVICE_URL` | Injected at deploy time | FQDN of user-service |
| `USER_SERVICE_API_KEY` | Key Vault: `internal-api-key` | Shared key for internal calls |
| `LOG_LEVEL` | Hardcoded | `info` |

---

## 7. Powering Services On and Off

To avoid Azure billing when the system is not in active use, all four Container Apps
can be scaled to zero replicas (paused) and restored at will. Data in Cosmos DB and
PostgreSQL is preserved in both states.

### Shut Down All Services

```bash
bash scripts/shutdown-services.sh
```

This sets `--min-replicas 0 --max-replicas 1` on every service. Each service will
scale to zero within 1–2 minutes after the last request drains.

### Power Up All Services

```bash
bash scripts/powerup-services.sh
```

This sets `--min-replicas 1` on every service, forcing at least one replica to
start immediately. Allow approximately **60 seconds** for all services to become
healthy before running tests.

### Power a Single Service On or Off

```bash
# Off
az containerapp update \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --min-replicas 0 --max-replicas 1

# On
az containerapp update \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --min-replicas 1 --max-replicas 3
```

### Checking Running Status

```bash
for SVC in user-service telemetry-service health-rules-service alerts-service; do
  REPLICAS=$(az containerapp replica list \
    --name "$SVC" \
    --resource-group rg-sentinel-health-engine \
    --query "length(@)" -o tsv 2>/dev/null || echo "0")
  echo "$SVC: $REPLICAS replica(s)"
done
```
