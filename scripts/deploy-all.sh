#!/usr/bin/env bash
# deploy-all.sh — builds and deploys all 6 services.
# Run from the sentinel-health-engine root directory.
# Usage: bash scripts/deploy-all.sh
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
ACR="crsentinelhe"
CONTAINER_ENV="cae-sentinel-he"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

TAG=$(date +%Y%m%d%H%M%S)

echo ">>> Logging into ACR..."
az acr login --name "$ACR"

# ── Build all images ──────────────────────────────────────────────────────────
echo ">>> Building images (tag: $TAG)..."
for SVC in user-service telemetry-service health-rules-service alerts-service analytics-service calendar-service; do
  echo "  Building $SVC..."
  docker build --no-cache \
    -t "${ACR}.azurecr.io/${SVC}:${TAG}" \
    -f "services/${SVC}/Dockerfile" \
    . &
done
wait
echo ">>> All images built."

# ── Push all images ───────────────────────────────────────────────────────────
echo ">>> Pushing images..."
for SVC in user-service telemetry-service health-rules-service alerts-service analytics-service calendar-service; do
  docker push "${ACR}.azurecr.io/${SVC}:${TAG}" &
done
wait
echo ">>> All images pushed."

# ── Deploy user-service first (others depend on it) ───────────────────────────
echo ">>> Deploying user-service..."
if az containerapp show --name user-service --resource-group "$RG" &>/dev/null; then
  az containerapp update \
    --name user-service \
    --resource-group "$RG" \
    --image "${ACR}.azurecr.io/user-service:${TAG}" \
    --replace-env-vars \
      "DATABASE_URL=$(get_secret postgres-database-url)" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "INTERNAL_API_KEY=$(get_secret internal-api-key)" \
      "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
      "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
      "RESET_PASSWORD_BASE_URL=sentinelhealth://reset-password" \
      "LOG_LEVEL=info"
else
  az containerapp create \
    --name user-service \
    --resource-group "$RG" \
    --environment "$CONTAINER_ENV" \
    --image "${ACR}.azurecr.io/user-service:${TAG}" \
    --registry-server "${ACR}.azurecr.io" \
    --min-replicas 1 \
    --max-replicas 3 \
    --ingress external \
    --target-port 8080 \
    --env-vars \
      "DATABASE_URL=$(get_secret postgres-database-url)" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "INTERNAL_API_KEY=$(get_secret internal-api-key)" \
      "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
      "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
      "RESET_PASSWORD_BASE_URL=sentinelhealth://reset-password" \
      "LOG_LEVEL=info"
fi

USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service --resource-group "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)"
echo "user-service URL: $USER_SERVICE_URL"

# ── Deploy remaining services ─────────────────────────────────────────────────
echo ">>> Deploying telemetry-service..."
az containerapp update \
  --name telemetry-service \
  --resource-group "$RG" \
  --image "${ACR}.azurecr.io/telemetry-service:${TAG}" \
  --replace-env-vars \
    "IOTHUB_EVENTHUB_CONNECTION_STRING=$(get_secret iothub-eventhub-connection-string)" \
    "IOTHUB_EVENTHUB_NAME=$(get_secret iothub-eventhub-name)" \
    "IOTHUB_CONSUMER_GROUP=telemetry-service" \
    "AZURE_STORAGE_CONNECTION_STRING=$(get_secret storage-connection-string)" \
    "CHECKPOINT_CONTAINER_NAME=iothub-checkpoints" \
    "SERVICE_BUS_CONNECTION_STRING=$(get_secret servicebus-connection-string)" \
    "TELEMETRY_TOPIC_NAME=telemetry-received" \
    "COSMOS_ENDPOINT=$(get_secret cosmos-endpoint)" \
    "COSMOS_KEY=$(get_secret cosmos-key)" \
    "COSMOS_DATABASE=sentinel-health" \
    "COSMOS_CONTAINER=telemetry" \
    "USER_SERVICE_URL=${USER_SERVICE_URL}" \
    "USER_SERVICE_API_KEY=$(get_secret internal-api-key)" \
    "LOG_LEVEL=info"

echo ">>> Deploying health-rules-service..."
az containerapp update \
  --name health-rules-service \
  --resource-group "$RG" \
  --image "${ACR}.azurecr.io/health-rules-service:${TAG}" \
  --replace-env-vars \
    "SERVICE_BUS_CONNECTION_STRING=$(get_secret servicebus-connection-string)" \
    "ANOMALY_TOPIC_NAME=anomaly-detected" \
    "TELEMETRY_TOPIC_NAME=telemetry-received" \
    "TELEMETRY_SUBSCRIPTION_NAME=health-rules-service" \
    "LOG_LEVEL=info"

echo ">>> Deploying alerts-service..."
az containerapp update \
  --name alerts-service \
  --resource-group "$RG" \
  --image "${ACR}.azurecr.io/alerts-service:${TAG}" \
  --replace-env-vars \
    "SERVICE_BUS_CONNECTION_STRING=$(get_secret servicebus-connection-string)" \
    "ANOMALY_TOPIC_NAME=anomaly-detected" \
    "ANOMALY_SUBSCRIPTION_NAME=alerts-service" \
    "COSMOS_ENDPOINT=$(get_secret cosmos-endpoint)" \
    "COSMOS_KEY=$(get_secret cosmos-key)" \
    "COSMOS_DATABASE=sentinel-health" \
    "COSMOS_ALERTS_CONTAINER=alerts" \
    "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
    "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
    "FIREBASE_CREDENTIALS_JSON=$(get_secret firebase-credentials-json)" \
    "USER_SERVICE_URL=${USER_SERVICE_URL}" \
    "USER_SERVICE_API_KEY=$(get_secret internal-api-key)" \
    "LOG_LEVEL=info"

echo ">>> Deploying analytics-service..."
if az containerapp show --name analytics-service --resource-group "$RG" &>/dev/null; then
  az containerapp update \
    --name analytics-service --resource-group "$RG" \
    --image "${ACR}.azurecr.io/analytics-service:${TAG}" \
    --replace-env-vars \
      "COSMOS_ENDPOINT=$(get_secret cosmos-endpoint)" \
      "COSMOS_KEY=$(get_secret cosmos-key)" \
      "COSMOS_DATABASE=sentinel-health" \
      "COSMOS_TELEMETRY_CONTAINER=telemetry" \
      "COSMOS_ALERTS_CONTAINER=alerts" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "LOG_LEVEL=info"
else
  az containerapp create \
    --name analytics-service --resource-group "$RG" \
    --environment "$CONTAINER_ENV" \
    --image "${ACR}.azurecr.io/analytics-service:${TAG}" \
    --registry-server "${ACR}.azurecr.io" \
    --min-replicas 1 --max-replicas 3 \
    --ingress external --target-port 8080 \
    --env-vars \
      "COSMOS_ENDPOINT=$(get_secret cosmos-endpoint)" \
      "COSMOS_KEY=$(get_secret cosmos-key)" \
      "COSMOS_DATABASE=sentinel-health" \
      "COSMOS_TELEMETRY_CONTAINER=telemetry" \
      "COSMOS_ALERTS_CONTAINER=alerts" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "LOG_LEVEL=info"
fi

echo ">>> Deploying calendar-service..."
if az containerapp show --name calendar-service --resource-group "$RG" &>/dev/null; then
  az containerapp update \
    --name calendar-service --resource-group "$RG" \
    --image "${ACR}.azurecr.io/calendar-service:${TAG}" \
    --replace-env-vars \
      "CALENDAR_DATABASE_URL=$(get_secret calendar-database-url)" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "USER_SERVICE_URL=${USER_SERVICE_URL}" \
      "USER_SERVICE_API_KEY=$(get_secret internal-api-key)" \
      "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
      "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
      "FIREBASE_CREDENTIALS_JSON=$(get_secret firebase-credentials-json)" \
      "LOG_LEVEL=info"
else
  az containerapp create \
    --name calendar-service --resource-group "$RG" \
    --environment "$CONTAINER_ENV" \
    --image "${ACR}.azurecr.io/calendar-service:${TAG}" \
    --registry-server "${ACR}.azurecr.io" \
    --min-replicas 1 --max-replicas 3 \
    --ingress external --target-port 8080 \
    --env-vars \
      "CALENDAR_DATABASE_URL=$(get_secret calendar-database-url)" \
      "JWT_SECRET=$(get_secret jwt-secret)" \
      "USER_SERVICE_URL=${USER_SERVICE_URL}" \
      "USER_SERVICE_API_KEY=$(get_secret internal-api-key)" \
      "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
      "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
      "FIREBASE_CREDENTIALS_JSON=$(get_secret firebase-credentials-json)" \
      "LOG_LEVEL=info"
fi

echo ""
echo "All 6 services deployed with tag: $TAG"
echo ""
echo "Next step — create the first doctor account:"
echo "  bash scripts/seed-first-doctor.sh \\"
echo "    --first-name \"Nombre\" \\"
echo "    --last-name \"Apellido\" \\"
echo "    --email \"doctor@example.com\" \\"
echo "    --password \"MyPassword123\""
