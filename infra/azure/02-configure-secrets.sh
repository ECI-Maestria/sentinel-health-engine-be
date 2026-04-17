#!/usr/bin/env bash
# =============================================================================
# Sentinel Health Engine — Configurar secrets en Key Vault
# Ejecuta DESPUÉS de 01-create-resources.sh
# =============================================================================
set -euo pipefail

RG="rg-sentinel-health-engine"
IOT_HUB="iothub-sentinel-he"
SB_NAMESPACE="sbns-sentinel-he"
COSMOS_ACCOUNT="cosmos-sentinel-he"
COSMOS_DB="sentinel-health"
STORAGE_ACCOUNT="stsentinelhe"
KV_NAME="kv-sentinel-he"
ACR_NAME="crsentinelhe"
ACS_NAME="acs-sentinel-he"

echo "============================================================"
echo "  Obteniendo connection strings y guardando en Key Vault"
echo "============================================================"

# IoT Hub — endpoint compatible con Event Hub
echo ""
echo ">>> IoT Hub Event Hub endpoint..."
IOTHUB_EH_NAME=$(az iot hub show \
  --name "$IOT_HUB" --resource-group "$RG" \
  --query "properties.eventHubEndpoints.events.path" -o tsv)

# Endpoint compatible con Event Hub (formato Endpoint=sb://, requerido por azeventhubs SDK)
IOTHUB_EH_ENDPOINT=$(az iot hub show \
  --name "$IOT_HUB" --resource-group "$RG" \
  --query "properties.eventHubEndpoints.events.endpoint" -o tsv)
IOTHUB_EH_KEY=$(az iot hub policy show \
  --name "service" --hub-name "$IOT_HUB" --resource-group "$RG" \
  --query "primaryKey" -o tsv)
IOTHUB_EH_CONN_STR="Endpoint=${IOTHUB_EH_ENDPOINT};SharedAccessKeyName=service;SharedAccessKey=${IOTHUB_EH_KEY}"

# Service Bus
echo ">>> Service Bus connection string..."
SB_CONN_STR=$(az servicebus namespace authorization-rule keys list \
  --name "RootManageSharedAccessKey" \
  --namespace-name "$SB_NAMESPACE" --resource-group "$RG" \
  --query "primaryConnectionString" -o tsv)

# Cosmos DB
echo ">>> Cosmos DB key..."
COSMOS_KEY=$(az cosmosdb keys list \
  --name "$COSMOS_ACCOUNT" --resource-group "$RG" \
  --query "primaryMasterKey" -o tsv)
COSMOS_ENDPOINT="https://${COSMOS_ACCOUNT}.documents.azure.com:443/"

# Storage Account
echo ">>> Storage Account connection string..."
STORAGE_CONN_STR=$(az storage account show-connection-string \
  --name "$STORAGE_ACCOUNT" --resource-group "$RG" \
  --query "connectionString" -o tsv)

# Azure Communication Services
echo ">>> ACS connection string..."
ACS_CONN_STR=$(az communication list-key \
  --name "$ACS_NAME" --resource-group "$RG" \
  --query "primaryConnectionString" -o tsv)

# ACR password
echo ">>> ACR password..."
ACR_PASSWORD=$(az acr credential show \
  --name "$ACR_NAME" --resource-group "$RG" \
  --query "passwords[0].value" -o tsv)

# Guardar en Key Vault
echo ""
echo ">>> Guardando secrets en Key Vault: $KV_NAME"
az keyvault secret set --vault-name "$KV_NAME" --name "iothub-eventhub-connection-string" --value "$IOTHUB_EH_CONN_STR" -o none
az keyvault secret set --vault-name "$KV_NAME" --name "iothub-eventhub-name"              --value "$IOTHUB_EH_NAME"     -o none
az keyvault secret set --vault-name "$KV_NAME" --name "servicebus-connection-string"       --value "$SB_CONN_STR"       -o none
az keyvault secret set --vault-name "$KV_NAME" --name "cosmos-endpoint"                    --value "$COSMOS_ENDPOINT"   -o none
az keyvault secret set --vault-name "$KV_NAME" --name "cosmos-key"                         --value "$COSMOS_KEY"        -o none
az keyvault secret set --vault-name "$KV_NAME" --name "storage-connection-string"          --value "$STORAGE_CONN_STR"  -o none
az keyvault secret set --vault-name "$KV_NAME" --name "acs-connection-string"              --value "$ACS_CONN_STR"      -o none
az keyvault secret set --vault-name "$KV_NAME" --name "acr-password"                       --value "$ACR_PASSWORD"      -o none
echo "✅ Secrets guardados"

# Imprimir valores para .env local
echo ""
echo "============================================================"
echo "  COPIA ESTOS VALORES A LOS ARCHIVOS .env DE CADA SERVICIO"
echo "============================================================"
echo ""
echo "# ---- services/telemetry-service/.env ----"
echo "IOTHUB_EVENTHUB_CONNECTION_STRING=$IOTHUB_EH_CONN_STR"
echo "IOTHUB_EVENTHUB_NAME=$IOTHUB_EH_NAME"
echo "IOTHUB_CONSUMER_GROUP=telemetry-service"
echo "AZURE_STORAGE_CONNECTION_STRING=$STORAGE_CONN_STR"
echo "CHECKPOINT_CONTAINER_NAME=iothub-checkpoints"
echo "SERVICE_BUS_CONNECTION_STRING=$SB_CONN_STR"
echo "TELEMETRY_TOPIC_NAME=telemetry-received"
echo "COSMOS_ENDPOINT=$COSMOS_ENDPOINT"
echo "COSMOS_KEY=$COSMOS_KEY"
echo "COSMOS_DATABASE=$COSMOS_DB"
echo "COSMOS_CONTAINER=telemetry"
echo "AUTHORIZED_DEVICES=mobile-gateway-01:TU-PATIENT-UUID-AQUI"
echo ""
echo "# ---- services/health-rules-service/.env ----"
echo "SERVICE_BUS_CONNECTION_STRING=$SB_CONN_STR"
echo "TELEMETRY_TOPIC_NAME=telemetry-received"
echo "TELEMETRY_SUBSCRIPTION_NAME=health-rules-service"
echo "ANOMALY_TOPIC_NAME=anomaly-detected"
echo ""
echo "# ---- services/alerts-service/.env ----"
echo "SERVICE_BUS_CONNECTION_STRING=$SB_CONN_STR"
echo "ANOMALY_TOPIC_NAME=anomaly-detected"
echo "ANOMALY_SUBSCRIPTION_NAME=alerts-service"
echo "COSMOS_ENDPOINT=$COSMOS_ENDPOINT"
echo "COSMOS_KEY=$COSMOS_KEY"
echo "COSMOS_DATABASE=$COSMOS_DB"
echo "COSMOS_ALERTS_CONTAINER=alerts"
echo "ACS_CONNECTION_STRING=$ACS_CONN_STR"
echo "ACS_SENDER_ADDRESS=DoNotReply@<tu-dominio>.azurecomm.net"
echo "FIREBASE_CREDENTIALS_FILE=/ruta/a/firebase-credentials.json"
echo "PATIENT_CONTACTS=TU-PATIENT-UUID:FCM-TOKEN:correo@ejemplo.com"
echo ""
echo ">>> SIGUIENTE PASO: Configura Firebase (ver guía), luego ejecuta 03-deploy-services.sh"
