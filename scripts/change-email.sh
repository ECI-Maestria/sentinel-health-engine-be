#!/usr/bin/env bash
# Cambia NUEVO_EMAIL por el que quieras usar:
# - dmurcia.cespedes@outlook.com
# - diego.murcia@mail.escuelaing.edu.co
# - diego.murcia@escuelaing.edu.co

NUEVO_EMAIL="diego.murcia@mail.escuelaing.edu.co"
KV="kv-sentinel-he"
RG="rg-sentinel-health-engine"
get_secret() { az keyvault secret show --vault-name "$KV" --name "$1" --query value -o tsv; }

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
    "PATIENT_CONTACTS=3ac0bc81-cb0f-44f9-b53a-fb45d6049d5e:FCM_PLACEHOLDER:$NUEVO_EMAIL" \
    "LOG_LEVEL=info"

echo "✅ Email actualizado a: $NUEVO_EMAIL"
