#!/usr/bin/env bash
# deploy-user-service.sh
# Builds, pushes, and deploys the user-service to Azure Container Apps.
# Run from the sentinel-health-engine root directory.
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
ACR="crsentinelhe"
CONTAINER_ENV="cae-sentinel-he"

get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

TAG=$(date +%Y%m%d%H%M%S)
IMAGE="${ACR}.azurecr.io/user-service:${TAG}"

echo ">>> Building image: $IMAGE..."
az acr login --name "$ACR"
docker build --no-cache \
  -t "$IMAGE" \
  -f services/user-service/Dockerfile \
  .

echo ">>> Pushing image..."
docker push "$IMAGE"

echo ">>> Reading secrets from Key Vault..."
DATABASE_URL=$(get_secret "postgres-database-url")
JWT_SECRET=$(get_secret "jwt-secret")
INTERNAL_API_KEY=$(get_secret "internal-api-key")
ACS_CONN=$(get_secret "acs-connection-string")
ACS_SENDER=$(az keyvault secret show --vault-name "$KV" --name "acs-sender-address" --query value -o tsv 2>/dev/null || echo "DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net")
IOTHUB_CONN=$(get_secret "iothub-connection-string")

echo ">>> Deploying user-service to Container Apps..."
# Create or update the container app
if az containerapp show --name user-service --resource-group "$RG" &>/dev/null; then
  az containerapp update \
    --name user-service \
    --resource-group "$RG" \
    --image "$IMAGE" \
    --replace-env-vars \
      "DATABASE_URL=${DATABASE_URL}" \
      "JWT_SECRET=${JWT_SECRET}" \
      "INTERNAL_API_KEY=${INTERNAL_API_KEY}" \
      "ACS_CONNECTION_STRING=${ACS_CONN}" \
      "ACS_SENDER_ADDRESS=${ACS_SENDER}" \
      "IOTHUB_CONNECTION_STRING=${IOTHUB_CONN}" \
      "ALLOWED_ORIGINS=*" \
      "LOG_LEVEL=info"
else
  az containerapp create \
    --name user-service \
    --resource-group "$RG" \
    --environment "$CONTAINER_ENV" \
    --image "$IMAGE" \
    --registry-server "${ACR}.azurecr.io" \
    --min-replicas 1 \
    --max-replicas 3 \
    --ingress external \
    --target-port 8080 \
    --env-vars \
      "DATABASE_URL=${DATABASE_URL}" \
      "JWT_SECRET=${JWT_SECRET}" \
      "INTERNAL_API_KEY=${INTERNAL_API_KEY}" \
      "ACS_CONNECTION_STRING=${ACS_CONN}" \
      "ACS_SENDER_ADDRESS=${ACS_SENDER}" \
      "IOTHUB_CONNECTION_STRING=${IOTHUB_CONN}" \
      "ALLOWED_ORIGINS=*" \
      "LOG_LEVEL=info"
fi

echo ">>> Getting user-service URL..."
USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service \
  --resource-group "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)"

echo ""
echo "✅ user-service deployed."
echo "URL: $USER_SERVICE_URL"
echo ""
echo ">>> Updating telemetry-service and alerts-service with user-service URL and API key..."
bash "$(dirname "$0")/update-telemetry-service.sh" "$USER_SERVICE_URL"
bash "$(dirname "$0")/update-alerts-with-userservice.sh" "$USER_SERVICE_URL"
