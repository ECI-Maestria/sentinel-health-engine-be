#!/usr/bin/env bash
# save as: pause-services.sh
RG="rg-sentinel-health-engine"

echo ">>> Escalando servicios a 0 réplicas..."

az containerapp update \
  --name telemetry-service \
  --resource-group "$RG" \
  --min-replicas 0 --max-replicas 1

az containerapp update \
  --name health-rules-service \
  --resource-group "$RG" \
  --min-replicas 0 --max-replicas 1

az containerapp update \
  --name alerts-service \
  --resource-group "$RG" \
  --min-replicas 0 --max-replicas 1

az containerapp update \
  --name user-service \
  --resource-group "$RG" \
  --min-replicas 0 --max-replicas 1

echo "✅ Servicios pausados. Cosmos DB, PostgreSQL y datos preservados."
