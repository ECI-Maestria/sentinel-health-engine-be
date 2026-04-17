#!/usr/bin/env bash
# update-alerts.sh — re-deploys alerts-service with latest image and env vars.
# Run from the sentinel-health-engine root directory.
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
ACR="crsentinelhe"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

TAG=$(date +%Y%m%d%H%M%S)
IMAGE="${ACR}.azurecr.io/alerts-service:${TAG}"

echo ">>> Building alerts-service image..."
az acr login --name "$ACR"
docker build --no-cache -t "$IMAGE" -f services/alerts-service/Dockerfile .
docker push "$IMAGE"

USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service --resource-group "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)"

echo ">>> Deploying alerts-service..."
az containerapp update \
  --name alerts-service \
  --resource-group "$RG" \
  --image "$IMAGE" \
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

echo "✅ alerts-service deployed with image $IMAGE"
