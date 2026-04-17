#!/usr/bin/env bash
# save as: resume-services.sh
RG="rg-sentinel-health-engine"

echo ">>> Reactivando servicios..."

az containerapp update \
  --name telemetry-service \
  --resource-group "$RG" \
  --min-replicas 1 --max-replicas 3

az containerapp update \
  --name health-rules-service \
  --resource-group "$RG" \
  --min-replicas 1 --max-replicas 3

az containerapp update \
  --name alerts-service \
  --resource-group "$RG" \
  --min-replicas 1 --max-replicas 2

az containerapp update \
  --name user-service \
  --resource-group "$RG" \
  --min-replicas 1 --max-replicas 3

echo "✅ Servicios activos. Espera ~60 segundos antes de hacer pruebas."
