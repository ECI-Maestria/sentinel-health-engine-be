#!/usr/bin/env bash
# update-telemetry-service.sh
# Updates telemetry-service env vars (called after user-service is deployed).
# Usage: ./update-telemetry-service.sh <USER_SERVICE_URL>
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

USER_SERVICE_URL="${1:-}"
if [[ -z "$USER_SERVICE_URL" ]]; then
  echo "ERROR: USER_SERVICE_URL argument is required"
  exit 1
fi

echo ">>> Updating telemetry-service..."
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

echo "✅ telemetry-service updated."
