#!/usr/bin/env bash
# fix-telemetry.sh — re-deploys telemetry-service with latest image and env vars.
# Run from the sentinel-health-engine root directory.
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
ACR="crsentinelhe"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

TAG=$(date +%Y%m%d%H%M%S)
IMAGE="${ACR}.azurecr.io/telemetry-service:${TAG}"

echo ">>> Building telemetry-service image..."
az acr login --name "$ACR"
docker build --no-cache -t "$IMAGE" -f services/telemetry-service/Dockerfile .
docker push "$IMAGE"

USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service --resource-group "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)"

echo ">>> Deploying telemetry-service..."
az containerapp update \
  --name telemetry-service \
  --resource-group "$RG" \
  --image "$IMAGE" \
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

echo "✅ telemetry-service deployed with image $IMAGE"
