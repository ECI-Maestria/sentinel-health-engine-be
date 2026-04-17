#!/usr/bin/env bash
# update-alerts-with-userservice.sh
# Updates alerts-service to use user-service (removes PATIENT_CONTACTS env var).
# Usage: ./update-alerts-with-userservice.sh <USER_SERVICE_URL>
set -euo pipefail

RG="rg-sentinel-health-engine"
KV="kv-sentinel-he"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

USER_SERVICE_URL="${1:-}"
if [[ -z "$USER_SERVICE_URL" ]]; then
  echo "ERROR: USER_SERVICE_URL argument is required"
  exit 1
fi

echo ">>> Updating alerts-service..."
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

echo "✅ alerts-service updated (PATIENT_CONTACTS replaced by user-service)."
