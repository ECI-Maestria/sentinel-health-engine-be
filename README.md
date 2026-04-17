# Sentinel Health Engine — Backend

Sistema de monitoreo de salud en tiempo real basado en microservicios Go desplegados en Azure Container Apps.

---

## Tabla de contenidos

1. [Estructura del proyecto](#1-estructura-del-proyecto)
2. [Arquitectura](#2-arquitectura)
3. [Correr en local con Docker](#3-correr-en-local-con-docker)
4. [Conectar con el frontend en local](#4-conectar-con-el-frontend-en-local)
5. [Montar los servicios en Azure](#5-montar-los-servicios-en-azure)
6. [Correr en local conectado a Azure](#6-correr-en-local-conectado-a-azure)
7. [Desplegar](#7-desplegar)
8. [Guía de uso de los servicios](#8-guía-de-uso-de-los-servicios)
9. [CI/CD](#9-cicd)
10. [Formato de mensajes IoT](#10-formato-de-mensajes-iot)

---

## 1. Estructura del proyecto

```
sentinel-health-engine-be/
├── services/
│   ├── user-service/          # Auth, pacientes, médicos — PostgreSQL + JWT
│   ├── telemetry-service/     # Ingesta de señales vitales desde IoT Hub
│   ├── health-rules-service/  # Evaluación de reglas clínicas — Service Bus
│   ├── alerts-service/        # Notificaciones push (Firebase) y email (ACS)
│   ├── analytics-service/     # Historial y estadísticas — Cosmos DB
│   └── calendar-service/      # Citas médicas — PostgreSQL
├── shared/
│   └── events/                # Tipos de eventos compartidos entre servicios
├── infra/
│   └── azure/
│       ├── 01-create-resources.sh    # Crea todos los recursos Azure (una vez)
│       ├── 02-configure-secrets.sh   # Extrae secrets y los guarda en Key Vault
│       ├── 03-deploy-services.sh     # Build, push y deploy a Container Apps
│       └── 04-device-management.sh   # Gestión de dispositivos IoT Hub
├── docs/
│   ├── diagrams/              # Diagramas de arquitectura (.excalidraw + .png)
│   ├── DEPLOYMENT_GUIDE.md
│   └── TESTING_GUIDE.md
├── .github/
│   └── workflows/
│       └── pipeline.yml       # CI/CD: Build → Test → SonarCloud → Deploy
├── docker-compose.yml         # Entorno local completo
├── go.work                    # Go workspace (todos los módulos)
├── Makefile                   # Comandos de build, test y Docker
└── sonar-project.properties   # Configuración de SonarCloud
```

Cada servicio sigue arquitectura hexagonal:

```
services/<nombre>/
├── cmd/server/main.go         # Punto de entrada
└── internal/
    ├── domain/                # Lógica de negocio pura (sin dependencias externas)
    ├── application/           # Casos de uso (orquestan domain + puertos)
    └── infrastructure/        # Adaptadores: DB, mensajería, HTTP
```

---

## 2. Arquitectura

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLIENTES                                   │
│   App Móvil (wearable gateway)          Frontend Web (React)        │
└───────────────┬─────────────────────────────────┬───────────────────┘
                │ IoT D2C messages                 │ REST / JWT
                ▼                                 ▼
        ┌───────────────┐                 ┌───────────────┐
        │   IoT Hub     │                 │  user-service │──── PostgreSQL
        └───────┬───────┘                 │   :8080       │
                │ Event Hub endpoint      └───────┬───────┘
                ▼                                 │ internal API
        ┌───────────────┐                         │
        │  telemetry-   │◄────────────────────────┘
        │   service     │  valida dispositivos
        │    :8081      │──── Cosmos DB (telemetría)
        └───────┬───────┘
                │ Service Bus: telemetry-received
                ▼
        ┌───────────────┐
        │ health-rules- │  evalúa reglas clínicas
        │   service     │  (sin persistencia propia)
        │    :8082      │
        └───────┬───────┘
                │ Service Bus: anomaly-detected
                ▼
        ┌───────────────┐
        │    alerts-    │──── Cosmos DB (alertas)
        │    service    │──── Firebase FCM (push)
        │     :8083     │──── ACS Email
        └───────────────┘

        ┌───────────────┐
        │  analytics-   │──── Cosmos DB (lectura)
        │   service     │
        │    :8084      │
        └───────────────┘

        ┌───────────────┐
        │   calendar-   │──── PostgreSQL
        │    service    │
        │     :8085     │
        └───────────────┘
```

**Flujo de telemetría completo:**
1. La app móvil envía señales vitales al **IoT Hub** vía MQTT/HTTPS
2. **telemetry-service** consume el Event Hub, valida el dispositivo contra **user-service**, persiste en **Cosmos DB** y publica en Service Bus
3. **health-rules-service** evalúa las reglas clínicas; si hay anomalía publica en Service Bus
4. **alerts-service** recibe la anomalía, persiste la alerta, y notifica vía **Firebase** y **ACS Email**

Diagramas detallados en [`docs/diagrams/`](docs/diagrams/).

---

## 3. Correr en local con Docker

El entorno local usa Docker Compose. Los servicios se conectan a **Azure real** (IoT Hub, Service Bus, Cosmos DB) a través de archivos `.env`, excepto PostgreSQL (local) y el checkpoint de Blob Storage (emulado con Azurite).

### Prerrequisitos

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) corriendo
- [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli) instalado
- Archivos `.env` configurados por servicio (ver [sección 6](#6-correr-en-local-conectado-a-azure))

### Levantar todo el stack

```bash
# Desde la raíz del proyecto
docker compose up --build
```

### Puertos expuestos

| Servicio              | Puerto local | URL                        |
|-----------------------|:------------:|----------------------------|
| user-service          | 8080         | http://localhost:8080      |
| telemetry-service     | 8081         | http://localhost:8081      |
| health-rules-service  | 8082         | http://localhost:8082      |
| alerts-service        | 8083         | http://localhost:8083      |
| analytics-service     | 8084         | http://localhost:8084      |
| calendar-service      | 8085         | http://localhost:8085      |
| PostgreSQL            | 5432         | postgres://localhost:5432  |
| Azurite (Blob)        | 10000        | http://localhost:10000     |

### Verificar que todo está sano

```bash
for port in 8080 8081 8082 8083 8084 8085; do
  echo "localhost:$port → $(curl -s http://localhost:$port/health | jq -r .status)"
done
```

### Documentación OpenAPI

Cada servicio con HTTP expone su API en `/docs`:

| Servicio          | URL                              |
|-------------------|----------------------------------|
| user-service      | http://localhost:8080/docs       |
| analytics-service | http://localhost:8084/docs       |
| calendar-service  | http://localhost:8085/docs       |

### Comandos útiles

```bash
# Ver logs de un servicio específico
docker compose logs -f telemetry-service

# Reconstruir solo un servicio
docker compose up --build user-service

# Detener todo sin borrar datos
docker compose stop

# Detener y borrar volúmenes (reset completo)
docker compose down -v
```

---

## 4. Conectar con el frontend en local

El frontend (React + Vite) se conecta al backend a través de la variable de entorno `VITE_API_URL`, que apunta a **user-service**.

### Paso 1 — Levantar el backend

```bash
# En el directorio del backend
docker compose up --build
```

### Paso 2 — Configurar el frontend

En el directorio del frontend, crea o edita el archivo `.env.local`:

```bash
# sentinel-health-engine-fe/.env.local
VITE_API_URL=http://localhost:8080
```

### Paso 3 — Levantar el frontend

```bash
# En el directorio del frontend
npm install
npm run dev
```

El frontend estará en `http://localhost:5173` y apuntará al backend local.

### Notas

- Los servicios `telemetry-service`, `health-rules-service` y `alerts-service` **no son llamados directamente por el frontend** — operan en background consumiendo Service Bus.
- `analytics-service` (`:8084`) y `calendar-service` (`:8085`) son llamados directamente si el frontend los usa. Asegúrate de que sus CORS permitan `http://localhost:5173`.

---

## 5. Montar los servicios en Azure

Ejecuta los scripts de `infra/azure/` **en orden**, una sola vez por entorno.

### Prerrequisitos

```bash
# Instalar extensiones de Azure CLI necesarias
az extension add --name containerapp
az extension add --name azure-iot

# Verificar login
az login
az account show
```

### Script 1 — Crear recursos

Crea todos los recursos Azure: IoT Hub, Service Bus, Cosmos DB, Storage, ACR, Container Apps Environment, Key Vault y ACS.

```bash
bash infra/azure/01-create-resources.sh
```

Recursos que crea:

| Recurso                     | Nombre                  |
|-----------------------------|-------------------------|
| Resource Group              | `rg-sentinel-health-engine` |
| IoT Hub                     | `iothub-sentinel-he`    |
| Service Bus Namespace       | `sbns-sentinel-he`      |
| Cosmos DB                   | `cosmos-sentinel-he`    |
| Storage Account             | `stsentinelhe`          |
| Container Registry          | `crsentinelhe`          |
| Container Apps Environment  | `cae-sentinel-he`       |
| Key Vault                   | `kv-sentinel-he`        |
| Communication Services      | `acs-sentinel-he`       |

> Tiempo estimado: **10–15 minutos**

### Script 2 — Configurar secrets

Extrae todos los connection strings y los guarda en Key Vault. Al final imprime los valores para los archivos `.env` locales.

```bash
bash infra/azure/02-configure-secrets.sh
```

### Script 3 — Desplegar servicios

Construye las imágenes Docker, las sube a ACR y crea las Container Apps.

```bash
# Antes de ejecutar, edita el script y completa:
# AUTHORIZED_DEVICES  → formato: deviceId:patientUUID
# PATIENT_CONTACTS    → formato: patientUUID:fcmToken:email
# ACS_SENDER          → tu dominio ACS verificado

bash infra/azure/03-deploy-services.sh
```

> Los valores de `AUTHORIZED_DEVICES` y `PATIENT_CONTACTS` se obtienen después de
> crear el primer médico y paciente vía la API de user-service.

### Obtener las URLs desplegadas

```bash
for SVC in user-service telemetry-service health-rules-service alerts-service analytics-service calendar-service; do
  FQDN=$(az containerapp show \
    --name "$SVC" --resource-group rg-sentinel-health-engine \
    --query properties.configuration.ingress.fqdn -o tsv 2>/dev/null)
  [ -n "$FQDN" ] && echo "$SVC → https://$FQDN"
done
```

### Gestión de dispositivos IoT

```bash
# Ver dispositivos registrados y obtener connection string para la app móvil
bash infra/azure/04-device-management.sh
```

---

## 6. Correr en local conectado a Azure

Esta modalidad corre los servicios Go localmente (vía Docker Compose) pero conectados a los recursos reales de Azure. Útil durante desarrollo cuando ya existe el entorno Azure.

### Crear los archivos `.env`

Ejecuta el script 02 para obtener los valores:

```bash
bash infra/azure/02-configure-secrets.sh
```

Luego crea un archivo `.env` por servicio con los valores impresos al final del script:

**`services/telemetry-service/.env`**
```env
IOTHUB_EVENTHUB_CONNECTION_STRING=<valor del script>
IOTHUB_EVENTHUB_NAME=<valor del script>
IOTHUB_CONSUMER_GROUP=telemetry-service
AZURE_STORAGE_CONNECTION_STRING=<valor del script>
CHECKPOINT_CONTAINER_NAME=iothub-checkpoints
SERVICE_BUS_CONNECTION_STRING=<valor del script>
TELEMETRY_TOPIC_NAME=telemetry-received
COSMOS_ENDPOINT=<valor del script>
COSMOS_KEY=<valor del script>
COSMOS_DATABASE=sentinel-health
COSMOS_CONTAINER=telemetry
AUTHORIZED_DEVICES=mobile-gateway-01:<patient-uuid>
LOG_LEVEL=info
```

**`services/health-rules-service/.env`**
```env
SERVICE_BUS_CONNECTION_STRING=<valor del script>
TELEMETRY_TOPIC_NAME=telemetry-received
TELEMETRY_SUBSCRIPTION_NAME=health-rules-service
ANOMALY_TOPIC_NAME=anomaly-detected
LOG_LEVEL=info
```

**`services/alerts-service/.env`**
```env
SERVICE_BUS_CONNECTION_STRING=<valor del script>
ANOMALY_TOPIC_NAME=anomaly-detected
ANOMALY_SUBSCRIPTION_NAME=alerts-service
COSMOS_ENDPOINT=<valor del script>
COSMOS_KEY=<valor del script>
COSMOS_DATABASE=sentinel-health
COSMOS_ALERTS_CONTAINER=alerts
ACS_CONNECTION_STRING=<valor del script>
ACS_SENDER_ADDRESS=DoNotReply@<tu-dominio>.azurecomm.net
FIREBASE_CREDENTIALS_FILE=/app/firebase-credentials.json
PATIENT_CONTACTS=<patient-uuid>:<fcm-token>:<email>
LOG_LEVEL=info
```

> Para alerts-service también necesitas el archivo `services/alerts-service/firebase-credentials.json`
> descargado desde la consola de Firebase (Project Settings → Service Accounts → Generate new private key).

**`services/user-service/.env`** — valores de Key Vault:
```bash
# Obtener desde Key Vault
az keyvault secret show --vault-name kv-sentinel-he --name postgres-database-url --query value -o tsv
az keyvault secret show --vault-name kv-sentinel-he --name jwt-secret --query value -o tsv
az keyvault secret show --vault-name kv-sentinel-he --name internal-api-key --query value -o tsv
```

```env
DATABASE_URL=<postgres connection string>
JWT_SECRET=<valor del Key Vault>
INTERNAL_API_KEY=<valor del Key Vault>
LOG_LEVEL=info
```

### Levantar

```bash
docker compose up --build
```

El checkpoint de Azurite (blob storage local) reemplaza al Storage Account de Azure solo para el telemetry-service. Todo lo demás usa Azure real.

---

## 7. Desplegar

### Despliegue automático (recomendado)

Cada `push` a la rama `main` dispara el pipeline completo de GitHub Actions:

```
🏗️ Build → 🧪 Test → 🔍 SonarCloud → 🚀 Deploy
```

El deploy actualiza las tres imágenes en ACR y reinicia las Container Apps automáticamente. Ver [sección CI/CD](#9-cicd) para detalles.

### Despliegue manual

```bash
# Variables
SERVICE="user-service"   # o telemetry-service, health-rules-service, alerts-service
ACR="crsentinelhe"
RG="rg-sentinel-health-engine"
TAG=$(git rev-parse --short HEAD)
IMAGE="${ACR}.azurecr.io/${SERVICE}:${TAG}"

# 1. Login a ACR
az acr login --name "$ACR"

# 2. Build desde la raíz del proyecto
docker build -t "$IMAGE" -f "services/${SERVICE}/Dockerfile" .

# 3. Push
docker push "$IMAGE"

# 4. Actualizar Container App
az containerapp update \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --image "$IMAGE"
```

### Verificar el despliegue

```bash
az containerapp revision list \
  --name "$SERVICE" \
  --resource-group "$RG" \
  --query "[0].{revision:name, estado:properties.runningState, imagen:properties.template.containers[0].image}" \
  --output table
```

### Apagar / encender servicios (ahorro de costos)

```bash
# Apagar (escalar a 0 réplicas)
for SVC in user-service telemetry-service health-rules-service alerts-service; do
  az containerapp update --name "$SVC" --resource-group rg-sentinel-health-engine \
    --min-replicas 0 --max-replicas 1
done

# Encender (mínimo 1 réplica)
for SVC in user-service telemetry-service health-rules-service alerts-service; do
  az containerapp update --name "$SVC" --resource-group rg-sentinel-health-engine \
    --min-replicas 1 --max-replicas 3
done
```

### Rollback

```bash
# Listar revisiones
az containerapp revision list --name "$SERVICE" --resource-group "$RG" --output table

# Activar revisión anterior
az containerapp revision activate \
  --name "$SERVICE" --resource-group "$RG" \
  --revision <nombre-revision-anterior>
```

---

## 8. Guía de uso de los servicios

Todos los ejemplos usan `http://localhost:8080` para el entorno local.
En Azure, reemplaza por la URL de la Container App correspondiente.

### Autenticación

**Login:**
```bash
curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@hospital.com","password":"TuPassword123!"}' | jq .
```

Respuesta:
```json
{
  "accessToken": "eyJhbGci...",
  "refreshToken": "eyJhbGci...",
  "expiresIn": 900
}
```

Guarda el token para las siguientes peticiones:
```bash
TOKEN="eyJhbGci..."
```

**Renovar token:**
```bash
curl -s -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":"<refresh_token>"}' | jq .
```

---

### user-service (`:8080`)

**Crear paciente** _(requiere rol DOCTOR)_:
```bash
curl -s -X POST http://localhost:8080/v1/patients \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "paciente@ejemplo.com",
    "firstName": "Juan",
    "lastName": "Pérez",
    "dateOfBirth": "1985-03-15",
    "deviceIdentifier": "mobile-gateway-01"
  }' | jq .
```

**Obtener perfil propio:**
```bash
curl -s http://localhost:8080/v1/users/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Listar pacientes** _(requiere rol DOCTOR)_:
```bash
curl -s http://localhost:8080/v1/patients \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Documentación completa:** http://localhost:8080/docs

---

### analytics-service (`:8084`)

**Historial de telemetría de un paciente:**
```bash
curl -s "http://localhost:8084/v1/patients/<patient-id>/telemetry?limit=50" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Alertas de un paciente:**
```bash
curl -s "http://localhost:8084/v1/patients/<patient-id>/alerts" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Documentación completa:** http://localhost:8084/docs

---

### calendar-service (`:8085`)

**Crear cita:**
```bash
curl -s -X POST http://localhost:8085/v1/appointments \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "patientId": "<patient-id>",
    "doctorId": "<doctor-id>",
    "scheduledAt": "2026-05-10T10:00:00Z",
    "notes": "Control mensual"
  }' | jq .
```

**Documentación completa:** http://localhost:8085/docs

---

### Health checks

```bash
for port in 8080 8081 8082 8083 8084 8085; do
  printf ":%d → %s\n" $port "$(curl -s http://localhost:$port/health | jq -r .status 2>/dev/null || echo 'no response')"
done
```

---

### APIs internas (service-to-service)

Protegidas con el header `X-Internal-API-Key`. Solo para uso entre servicios.

```bash
INTERNAL_KEY=$(az keyvault secret show --vault-name kv-sentinel-he \
  --name internal-api-key --query value -o tsv)

# Validar dispositivo (usado por telemetry-service)
curl -s http://localhost:8080/v1/internal/devices/mobile-gateway-01 \
  -H "X-Internal-API-Key: $INTERNAL_KEY" | jq .

# Obtener contactos de un paciente (usado por alerts-service)
curl -s http://localhost:8080/v1/internal/patients/<patient-id>/contacts \
  -H "X-Internal-API-Key: $INTERNAL_KEY" | jq .
```

---

## 9. CI/CD

El pipeline está definido en [`.github/workflows/pipeline.yml`](.github/workflows/pipeline.yml) y corre en GitHub Actions.

### Etapas

```
🏗️ Build ──► 🧪 Test ──► 🔍 SonarCloud ──► 🚀 Deploy
```

| Etapa | Qué hace | Cuándo corre |
|-------|----------|--------------|
| 🏗️ **Build** | `go build ./...` para los 3 servicios en paralelo | PRs + push a `main` |
| 🧪 **Test** | `go test ./...` con reporte de cobertura | Solo si Build pasa |
| 🔍 **SonarCloud** | Análisis estático + cobertura en sonarcloud.io | Solo si Test pasa |
| 🚀 **Deploy** | Build imagen → push a ACR → `az containerapp update` | Solo si Sonar pasa **y** es push a `main` |

Si cualquier etapa falla, las siguientes se cancelan automáticamente (`fail-fast: true`).

### Secretos requeridos en GitHub

| Secret | Descripción |
|--------|-------------|
| `AZURE_CREDENTIALS` | JSON del service principal (ver abajo) |
| `SONAR_TOKEN` | Token de SonarCloud (Account → Security → Generate Token) |

```bash
# Crear el service principal (ya ejecutado durante el setup inicial)
az ad sp create-for-rbac \
  --name "github-sentinel-cicd" \
  --role contributor \
  --scopes /subscriptions/<SUB_ID>/resourceGroups/rg-sentinel-health-engine \
  --sdk-auth
```

### Ver resultados

- **GitHub Actions:** `github.com/ECI-Maestria/sentinel-health-engine-be/actions`
- **SonarCloud:** `sonarcloud.io/project/overview?id=ECI-Maestria_sentinel-health-engine-be`

### Agregar un nuevo servicio al pipeline

1. Añade el nombre del servicio a la `matrix` en los jobs `build`, `test` y `deploy` de `pipeline.yml`
2. Añade el `coverage_out` correspondiente en el job `test`
3. Actualiza `sonar.sources` en `sonar-project.properties`

---

## 10. Formato de mensajes IoT

La app móvil actúa como gateway entre el wearable y el sistema. Envía mensajes al **IoT Hub** usando el device `mobile-gateway-01`.

### Credenciales del dispositivo

```bash
# Obtener connection string para el dispositivo (usar en la app móvil)
bash infra/azure/04-device-management.sh
```

### Formato del payload

```json
{
  "deviceId": "mobile-gateway-01",
  "heartRate": 75,
  "spO2": 98.5,
  "timestamp": "2026-04-17T10:00:00Z"
}
```

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `deviceId` | string | ID registrado en IoT Hub. Debe coincidir con el dispositivo autorizado |
| `heartRate` | int | Frecuencia cardíaca en bpm. Rango válido: 0–300 |
| `spO2` | float | Saturación de oxígeno en %. Rango válido: 0–100 |
| `timestamp` | string | ISO 8601 UTC. Momento de la medición en el dispositivo |

### Simular un mensaje de prueba

```bash
az iot device send-d2c-message \
  --hub-name iothub-sentinel-he \
  --device-id mobile-gateway-01 \
  --data '{"deviceId":"mobile-gateway-01","heartRate":105,"spO2":91.0,"timestamp":"2026-04-17T12:00:00Z"}'
```

### Reglas clínicas por defecto

El `health-rules-service` evalúa automáticamente estas reglas en cada lectura:

| Condición | Métrica | Umbral | Severidad |
|-----------|---------|--------|-----------|
| SpO2 baja | spO2 < | 95% | ⚠️ WARNING |
| SpO2 crítica | spO2 < | 90% | 🚨 CRITICAL |
| Taquicardia | heartRate > | 100 bpm | ⚠️ WARNING |
| Taquicardia severa | heartRate > | 130 bpm | 🚨 CRITICAL |
| Bradicardia | heartRate < | 50 bpm | ⚠️ WARNING |
| Bradicardia severa | heartRate < | 40 bpm | 🚨 CRITICAL |

Cuando se detecta una anomalía, `alerts-service` envía automáticamente una notificación push (Firebase) y un correo (ACS) a los contactos del paciente.

---

## Prerrequisitos generales

| Herramienta | Versión mínima | Verificar |
|-------------|:--------------:|-----------|
| Go | 1.23 | `go version` |
| Docker Desktop | 4.x | `docker --version` |
| Azure CLI | 2.55+ | `az --version` |
| Git | cualquiera | `git --version` |

```bash
# Extensiones de Azure CLI necesarias
az extension add --name containerapp
az extension add --name azure-iot
```
