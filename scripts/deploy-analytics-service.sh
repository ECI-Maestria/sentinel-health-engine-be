#!/usr/bin/env bash
# deploy-analytics-service.sh
# Builds, pushes, and deploys the analytics-service to Azure Container Apps.
# Run from the sentinel-health-engine root directory.
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
ACR="crsentinelhe"
CONTAINER_ENV="cae-sentinel-he"

get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

TAG=$(date +%Y%m%d%H%M%S)
IMAGE="${ACR}.azurecr.io/analytics-service:${TAG}"

echo ">>> Building image: $IMAGE..."
az acr login --name "$ACR"
docker build --no-cache \
  -t "$IMAGE" \
  -f services/analytics-service/Dockerfile \
  .

echo ">>> Pushing image..."
docker push "$IMAGE"

echo ">>> Reading secrets from Key Vault..."
COSMOS_ENDPOINT=$(get_secret "cosmos-endpoint")
COSMOS_KEY=$(get_secret "cosmos-key")
JWT_SECRET=$(get_secret "jwt-secret")

echo ">>> Deploying analytics-service to Container Apps..."
if az containerapp show --name analytics-service --resource-group "$RG" &>/dev/null; then
  az containerapp update \
    --name analytics-service \
    --resource-group "$RG" \
    --image "$IMAGE" \
    --replace-env-vars \
      "COSMOS_ENDPOINT=${COSMOS_ENDPOINT}" \
      "COSMOS_KEY=${COSMOS_KEY}" \
      "COSMOS_DATABASE=sentinel-health" \
      "COSMOS_TELEMETRY_CONTAINER=telemetry" \
      "COSMOS_ALERTS_CONTAINER=alerts" \
      "JWT_SECRET=${JWT_SECRET}" \
      "ALLOWED_ORIGINS=*" \
      "LOG_LEVEL=info"
else
  az containerapp create \
    --name analytics-service \
    --resource-group "$RG" \
    --environment "$CONTAINER_ENV" \
    --image "$IMAGE" \
    --registry-server "${ACR}.azurecr.io" \
    --min-replicas 1 \
    --max-replicas 3 \
    --ingress external \
    --target-port 8080 \
    --env-vars \
      "COSMOS_ENDPOINT=${COSMOS_ENDPOINT}" \
      "COSMOS_KEY=${COSMOS_KEY}" \
      "COSMOS_DATABASE=sentinel-health" \
      "COSMOS_TELEMETRY_CONTAINER=telemetry" \
      "COSMOS_ALERTS_CONTAINER=alerts" \
      "JWT_SECRET=${JWT_SECRET}" \
      "ALLOWED_ORIGINS=*" \
      "LOG_LEVEL=info"
fi

echo ""
echo "analytics-service deployed."
echo "Image tag: $TAG"
echo "URL: https://analytics-service.yellowmeadow-4dfba13a.centralus.azurecontainerapps.io"
