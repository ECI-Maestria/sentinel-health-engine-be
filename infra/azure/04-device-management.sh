#!/usr/bin/env bash
# =============================================================================
# Sentinel Health Engine — Gestión de dispositivos IoT
# Registra dispositivos y obtén connection strings para la app móvil
# =============================================================================
set -euo pipefail

RG="rg-sentinel-health-engine"
IOT_HUB="iothub-sentinel-he"

echo "=========================================="
echo " Dispositivos registrados en IoT Hub"
echo "=========================================="
az iot hub device-identity list \
  --hub-name "$IOT_HUB" --resource-group "$RG" \
  --query "[].{DeviceId:deviceId, Status:status, AuthType:authentication.type}" \
  -o table

echo ""
echo "Connection string para 'mobile-gateway-01' (usar en la app móvil):"
az iot hub device-identity connection-string show \
  --hub-name "$IOT_HUB" \
  --device-id "mobile-gateway-01" \
  --resource-group "$RG" \
  --query "connectionString" -o tsv

echo ""
echo "Para registrar un nuevo dispositivo:"
echo "  az iot hub device-identity create --hub-name $IOT_HUB --device-id <ID> --resource-group $RG"
echo ""
echo "Formato JSON que DEBE enviar la app móvil al IoT Hub:"
cat << 'PAYLOAD'
{
  "deviceId": "mobile-gateway-01",
  "heartRate": 75,
  "spO2": 98.5,
  "timestamp": "2026-03-30T10:00:00Z"
}
PAYLOAD
echo ""
echo "Para simular un mensaje de prueba desde la CLI:"
echo "  az iot device send-d2c-message --hub-name $IOT_HUB --device-id mobile-gateway-01 \\"
echo "    --data '{\"deviceId\":\"mobile-gateway-01\",\"heartRate\":105,\"spO2\":91.0,\"timestamp\":\"2026-03-30T12:00:00Z\"}'"
