#!/usr/bin/env bash
# update-env.sh — updates env vars on all services without rebuilding images.
# Useful after rotating secrets or changing configuration.
# Run from the sentinel-health-engine root directory.
set -euo pipefail

KV="kv-sentinel-he"
RG="rg-sentinel-health-engine"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service --resource-group "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)"

echo ">>> Updating telemetry-service env vars..."
az containerapp update \
  --name telemetry-service \
  --resource-group "$RG" \
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

echo ">>> Updating alerts-service env vars..."
az containerapp update \
  --name alerts-service \
  --resource-group "$RG" \
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

echo ">>> Updating user-service env vars..."
az containerapp update \
  --name user-service \
  --resource-group "$RG" \
  --replace-env-vars \
    "DATABASE_URL=$(get_secret postgres-database-url)" \
    "JWT_SECRET=$(get_secret jwt-secret)" \
    "INTERNAL_API_KEY=$(get_secret internal-api-key)" \
    "ACS_CONNECTION_STRING=$(get_secret acs-connection-string)" \
    "ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net" \
    "LOG_LEVEL=info"

echo "All env vars updated."
