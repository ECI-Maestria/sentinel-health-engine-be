#!/usr/bin/env bash
# =============================================================================
# Sentinel Health Engine — Build, Push y Deploy a Azure Container Apps
# Ejecuta DESPUÉS de configurar los .env y 02-configure-secrets.sh
# =============================================================================
set -euo pipefail

RG="rg-sentinel-health-engine"
ACR_NAME="crsentinelhe"
CAE_NAME="cae-sentinel-he"
KV_NAME="kv-sentinel-he"
TAG="${TAG:-latest}"
ACR_SERVER="${ACR_NAME}.azurecr.io"

# Helper para leer secrets del Key Vault
get_secret() { az keyvault secret show --vault-name "$KV_NAME" --name "$1" --query "value" -o tsv; }

echo ">>> Login a Container Registry..."
az acr login --name "$ACR_NAME"

# Build y push de las 3 imágenes
for SVC in telemetry-service health-rules-service alerts-service; do
  echo ">>> Build & push $SVC..."
  docker build -t "${ACR_SERVER}/${SVC}:${TAG}" -f "services/${SVC}/Dockerfile" .
  docker push "${ACR_SERVER}/${SVC}:${TAG}"
done

# Recuperar secrets
SB_CONN=$(get_secret "servicebus-connection-string")
COSMOS_ENDPOINT=$(get_secret "cosmos-endpoint")
COSMOS_KEY=$(get_secret "cosmos-key")
IOTHUB_EH_CONN=$(get_secret "iothub-eventhub-connection-string")
IOTHUB_EH_NAME=$(get_secret "iothub-eventhub-name")
STORAGE_CONN=$(get_secret "storage-connection-string")
ACS_CONN=$(get_secret "acs-connection-string")
ACR_PASSWORD=$(get_secret "acr-password")
FIREBASE_CREDS=$(get_secret "firebase-credentials-json")

# ── Valores que debes personalizar ───────────────────────────────────────────
# Formato: deviceId:patientUUID  (el patientUUID lo defines tú, debe ser un UUID v4)
AUTHORIZED_DEVICES="${AUTHORIZED_DEVICES:-mobile-gateway-01:REEMPLAZA-PATIENT-UUID}"
# Formato: patientUUID:fcmToken:email
PATIENT_CONTACTS="${PATIENT_CONTACTS:-REEMPLAZA-PATIENT-UUID:REEMPLAZA-FCM-TOKEN:REEMPLAZA-EMAIL}"
# Dominio de ACS: Azure Portal → Communication Services → Email → Domains
ACS_SENDER="${ACS_SENDER_ADDRESS:-DoNotReply@REEMPLAZA.azurecomm.net}"

# ── telemetry-service ─────────────────────────────────────────────────────────
echo ""
echo ">>> Deploying telemetry-service..."
az containerapp create \
  --name "telemetry-service" \
  --resource-group "$RG" \
  --environment "$CAE_NAME" \
  --image "${ACR_SERVER}/telemetry-service:${TAG}" \
  --registry-server "$ACR_SERVER" \
  --registry-username "$ACR_NAME" \
  --registry-password "$ACR_PASSWORD" \
  --target-port 8080 \
  --ingress internal \
  --min-replicas 1 --max-replicas 3 \
  --cpu 0.25 --memory 0.5Gi \
  --env-vars \
    "IOTHUB_EVENTHUB_CONNECTION_STRING=$IOTHUB_EH_CONN" \
    "IOTHUB_EVENTHUB_NAME=$IOTHUB_EH_NAME" \
    "IOTHUB_CONSUMER_GROUP=telemetry-service" \
    "AZURE_STORAGE_CONNECTION_STRING=$STORAGE_CONN" \
    "CHECKPOINT_CONTAINER_NAME=iothub-checkpoints" \
    "SERVICE_BUS_CONNECTION_STRING=$SB_CONN" \
    "TELEMETRY_TOPIC_NAME=telemetry-received" \
    "COSMOS_ENDPOINT=$COSMOS_ENDPOINT" \
    "COSMOS_KEY=$COSMOS_KEY" \
    "COSMOS_DATABASE=sentinel-health" \
    "COSMOS_CONTAINER=telemetry" \
    "AUTHORIZED_DEVICES=$AUTHORIZED_DEVICES" \
    "LOG_LEVEL=info"
echo "✅ telemetry-service desplegado"

# ── health-rules-service ──────────────────────────────────────────────────────
echo ""
echo ">>> Deploying health-rules-service..."
az containerapp create \
  --name "health-rules-service" \
  --resource-group "$RG" \
  --environment "$CAE_NAME" \
  --image "${ACR_SERVER}/health-rules-service:${TAG}" \
  --registry-server "$ACR_SERVER" \
  --registry-username "$ACR_NAME" \
  --registry-password "$ACR_PASSWORD" \
  --target-port 8080 \
  --ingress internal \
  --min-replicas 1 --max-replicas 3 \
  --cpu 0.25 --memory 0.5Gi \
  --env-vars \
    "SERVICE_BUS_CONNECTION_STRING=$SB_CONN" \
    "TELEMETRY_TOPIC_NAME=telemetry-received" \
    "TELEMETRY_SUBSCRIPTION_NAME=health-rules-service" \
    "ANOMALY_TOPIC_NAME=anomaly-detected" \
    "LOG_LEVEL=info"
echo "✅ health-rules-service desplegado"

# ── alerts-service ────────────────────────────────────────────────────────────
echo ""
echo ">>> Deploying alerts-service..."
az containerapp create \
  --name "alerts-service" \
  --resource-group "$RG" \
  --environment "$CAE_NAME" \
  --image "${ACR_SERVER}/alerts-service:${TAG}" \
  --registry-server "$ACR_SERVER" \
  --registry-username "$ACR_NAME" \
  --registry-password "$ACR_PASSWORD" \
  --target-port 8080 \
  --ingress internal \
  --min-replicas 1 --max-replicas 2 \
  --cpu 0.5 --memory 1.0Gi \
  --env-vars \
    "SERVICE_BUS_CONNECTION_STRING=$SB_CONN" \
    "ANOMALY_TOPIC_NAME=anomaly-detected" \
    "ANOMALY_SUBSCRIPTION_NAME=alerts-service" \
    "COSMOS_ENDPOINT=$COSMOS_ENDPOINT" \
    "COSMOS_KEY=$COSMOS_KEY" \
    "COSMOS_DATABASE=sentinel-health" \
    "COSMOS_ALERTS_CONTAINER=alerts" \
    "ACS_CONNECTION_STRING=$ACS_CONN" \
    "ACS_SENDER_ADDRESS=$ACS_SENDER" \
    "FIREBASE_CREDENTIALS_JSON=$FIREBASE_CREDS" \
    "PATIENT_CONTACTS=$PATIENT_CONTACTS" \
    "AUTHORIZED_DEVICES=mobile-gateway-01:3ac0bc81-cb0f-44f9-b53a-fb45d6049d5e" \
    "LOG_LEVEL=info"
echo "✅ alerts-service desplegado"

echo ""
echo "============================================================"
echo "  ✅ TODOS LOS SERVICIOS DESPLEGADOS"
echo ""
echo "  Verificar health checks:"
echo "  az containerapp show -n telemetry-service -g $RG --query 'properties.latestRevisionFqdn' -o tsv"
echo "  az containerapp logs show -n telemetry-service -g $RG --follow"
echo "============================================================"
