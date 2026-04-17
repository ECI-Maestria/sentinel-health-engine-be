#!/usr/bin/env bash
# =============================================================================
# Sentinel Health Engine — Azure Infrastructure Setup
# Ejecuta este script UNA sola vez para crear todos los recursos.
# Región: centralus (US Central)
# =============================================================================
set -euo pipefail

# --- Variables ----------------------------------------------------------------
LOCATION="centralus"
RG="rg-sentinel-health-engine"
IOT_HUB="iothub-sentinel-he"
SB_NAMESPACE="sbns-sentinel-he"
COSMOS_ACCOUNT="cosmos-sentinel-he"
COSMOS_DB="sentinel-health"
ACR_NAME="crsentinelhe"          # Solo alfanumérico, globalmente único
CAE_NAME="cae-sentinel-he"
KV_NAME="kv-sentinel-he"
ACS_NAME="acs-sentinel-he"
STORAGE_ACCOUNT="stsentinelhe"   # Máx 24 chars, solo minúsculas

echo "============================================================"
echo "  Sentinel Health Engine — Creación de recursos Azure"
echo "  Región: $LOCATION"
echo "============================================================"

# PASO 0: Verificar login
echo ""
echo ">>> Verificando sesión Azure CLI..."
az account show --query "{Suscripción:name, ID:id}" -o table
echo ""
read -p "¿Es la suscripción correcta? Presiona ENTER para continuar, Ctrl+C para cancelar..."

# PASO 1: Resource Group
echo ""
echo ">>> PASO 1: Creando Resource Group..."
az group create \
  --name "$RG" \
  --location "$LOCATION" \
  --tags project=sentinel-health-engine env=dev
echo "✅ Resource Group: $RG"

# PASO 2: IoT Hub (S1 Standard — necesario para consumer groups y routing)
echo ""
echo ">>> PASO 2: Creando Azure IoT Hub (SKU S1)..."
az iot hub create \
  --name "$IOT_HUB" \
  --resource-group "$RG" \
  --location "$LOCATION" \
  --sku S1 \
  --unit 1
echo "✅ IoT Hub: $IOT_HUB"

# Consumer group dedicado para el Telemetry Service
az iot hub consumer-group create \
  --hub-name "$IOT_HUB" \
  --name "telemetry-service" \
  --resource-group "$RG"
echo "✅ Consumer group 'telemetry-service' creado"

# Registro del dispositivo (la app móvil actúa como gateway)
az iot hub device-identity create \
  --hub-name "$IOT_HUB" \
  --device-id "mobile-gateway-01" \
  --resource-group "$RG"
echo "✅ Dispositivo 'mobile-gateway-01' registrado"

# PASO 3: Service Bus (Standard para topics/subscriptions)
echo ""
echo ">>> PASO 3: Creando Azure Service Bus..."
az servicebus namespace create \
  --name "$SB_NAMESPACE" \
  --resource-group "$RG" \
  --location "$LOCATION" \
  --sku Standard
echo "✅ Service Bus Namespace: $SB_NAMESPACE"

# Topics
az servicebus topic create --name "telemetry-received" --namespace-name "$SB_NAMESPACE" --resource-group "$RG" --default-message-time-to-live "P1D"
az servicebus topic create --name "anomaly-detected"   --namespace-name "$SB_NAMESPACE" --resource-group "$RG" --default-message-time-to-live "P1D"
az servicebus topic create --name "alert-created"      --namespace-name "$SB_NAMESPACE" --resource-group "$RG" --default-message-time-to-live "P7D"
echo "✅ Topics creados: telemetry-received, anomaly-detected, alert-created"

# Subscriptions
az servicebus topic subscription create \
  --name "health-rules-service" --topic-name "telemetry-received" \
  --namespace-name "$SB_NAMESPACE" --resource-group "$RG" \
  --max-delivery-count 5 --enable-dead-lettering-on-message-expiration true

az servicebus topic subscription create \
  --name "alerts-service" --topic-name "anomaly-detected" \
  --namespace-name "$SB_NAMESPACE" --resource-group "$RG" \
  --max-delivery-count 5 --enable-dead-lettering-on-message-expiration true
echo "✅ Subscriptions creadas"

# PASO 4: Cosmos DB Serverless (ideal para PoC — pagas por request)
echo ""
echo ">>> PASO 4: Creando Azure Cosmos DB (Serverless)..."
az cosmosdb create \
  --name "$COSMOS_ACCOUNT" \
  --resource-group "$RG" \
  --locations regionName="$LOCATION" failoverPriority=0 isZoneRedundant=False \
  --capabilities EnableServerless \
  --default-consistency-level "Session"
echo "✅ Cosmos DB: $COSMOS_ACCOUNT"

az cosmosdb sql database create --account-name "$COSMOS_ACCOUNT" --resource-group "$RG" --name "$COSMOS_DB"

az cosmosdb sql container create --account-name "$COSMOS_ACCOUNT" --resource-group "$RG" \
  --database-name "$COSMOS_DB" --name "telemetry"    --partition-key-path "/deviceId"
az cosmosdb sql container create --account-name "$COSMOS_ACCOUNT" --resource-group "$RG" \
  --database-name "$COSMOS_DB" --name "health-rules" --partition-key-path "/patientId"
az cosmosdb sql container create --account-name "$COSMOS_ACCOUNT" --resource-group "$RG" \
  --database-name "$COSMOS_DB" --name "alerts"       --partition-key-path "/patientId"
echo "✅ Containers creados: telemetry, health-rules, alerts"

# PASO 5: Storage Account (checkpointing del Event Hub)
echo ""
echo ">>> PASO 5: Creando Storage Account..."
az storage account create \
  --name "$STORAGE_ACCOUNT" \
  --resource-group "$RG" \
  --location "$LOCATION" \
  --sku Standard_LRS \
  --kind StorageV2 \
  --min-tls-version TLS1_2

STORAGE_KEY=$(az storage account keys list \
  --account-name "$STORAGE_ACCOUNT" --resource-group "$RG" \
  --query "[0].value" -o tsv)

az storage container create \
  --name "iothub-checkpoints" \
  --account-name "$STORAGE_ACCOUNT" \
  --account-key "$STORAGE_KEY"
echo "✅ Storage: $STORAGE_ACCOUNT / iothub-checkpoints"

# PASO 6: Container Registry
echo ""
echo ">>> PASO 6: Creando Azure Container Registry..."
az acr create \
  --name "$ACR_NAME" \
  --resource-group "$RG" \
  --location "$LOCATION" \
  --sku Basic \
  --admin-enabled true
echo "✅ Container Registry: $ACR_NAME"

# PASO 7: Container Apps Environment
echo ""
echo ">>> PASO 7: Creando Container Apps Environment..."
az containerapp env create \
  --name "$CAE_NAME" \
  --resource-group "$RG" \
  --location "$LOCATION"
echo "✅ Container Apps Environment: $CAE_NAME"

# PASO 8: Key Vault
echo ""
echo ">>> PASO 8: Creando Key Vault..."
az keyvault create \
  --name "$KV_NAME" \
  --resource-group "$RG" \
  --location "$LOCATION" \
  --sku standard \
  --retention-days 7
echo "✅ Key Vault: $KV_NAME"

# Asignar rol Key Vault Secrets Officer al usuario actual (requerido con modelo RBAC)
CURRENT_USER_OID=$(az ad signed-in-user show --query id -o tsv)
KV_RESOURCE_ID=$(az keyvault show --name "$KV_NAME" --resource-group "$RG" --query id -o tsv)
az role assignment create \
  --role "Key Vault Secrets Officer" \
  --assignee "$CURRENT_USER_OID" \
  --scope "$KV_RESOURCE_ID" \
  --output none
echo "✅ Rol 'Key Vault Secrets Officer' asignado a tu usuario"

# PASO 9: Azure Communication Services
echo ""
echo ">>> PASO 9: Creando Azure Communication Services..."
az communication create \
  --name "$ACS_NAME" \
  --resource-group "$RG" \
  --location "global" \
  --data-location "United States"
echo "✅ Communication Services: $ACS_NAME"

echo ""
echo "============================================================"
echo "  ✅ TODOS LOS RECURSOS CREADOS"
echo "  Ejecuta: bash 02-configure-secrets.sh"
echo "============================================================"
