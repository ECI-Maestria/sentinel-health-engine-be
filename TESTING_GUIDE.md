# Guía de pruebas — Sentinel Health Engine

Paso a paso completo para correr y probar el backend (6 servicios) en **local** y en **Azure**.

---

## Índice

1. [Arquitectura](#1-arquitectura)
2. [Prerequisitos](#2-prerequisitos)
3. [Configuración local](#3-configuración-local)
4. [Levantar en local](#4-levantar-en-local)
5. [Pruebas en local](#5-pruebas-en-local)
6. [Despliegue en Azure](#6-despliegue-en-azure)
7. [Pruebas en Azure](#7-pruebas-en-azure)
8. [Referencia de puertos y URLs](#8-referencia-de-puertos-y-urls)
9. [Variables de entorno](#9-variables-de-entorno)

---

## 1. Arquitectura

```
[IoT Device] ──MQTT──► [Azure IoT Hub]
                              │
                    [telemetry-service :8081]  ──► Cosmos DB (telemetry)
                              │ Service Bus: telemetry-received
                    [health-rules-service :8082]
                              │ Service Bus: anomaly-detected
                    [alerts-service :8083]  ──► Cosmos DB (alerts)
                              │                   Firebase push + ACS email
                              └──► [user-service :8080]  ──► PostgreSQL (sentinel_users)

[App Mobile / Web]
    ├── user-service      :8080  → auth, pacientes, cuidadores, dispositivos, dashboard
    ├── analytics-service :8084  → vitales históricos, alertas, reportes PDF
    └── calendar-service  :8085  → citas, medicamentos, recordatorios

[PostgreSQL]
    ├── sentinel_users    (user-service)
    └── sentinel_calendar (calendar-service)
```

**Contenedores Docker locales:** 6 servicios Go + postgres + azurite
**Azure (siempre real):** IoT Hub, Service Bus, Cosmos DB, ACS, Firebase

---

## 2. Prerequisitos

### Herramientas locales

| Herramienta | Versión | Instalación |
|---|---|---|
| Docker Desktop | 24+ | https://www.docker.com/products/docker-desktop |
| Azure CLI | 2.50+ | `winget install Microsoft.AzureCLI` |
| `curl` + `jq` | cualquiera | `winget install jqlang.jq` |

> Go **NO** es necesario localmente — todo compila dentro del contenedor.

### Recursos Azure requeridos
```
rg-sentinel-health-engine
├── iothub-sentinel-he         (IoT Hub)
├── sbns-sentinel-he           (Service Bus)
│   ├── topic: telemetry-received → subscription: health-rules-service
│   └── topic: anomaly-detected   → subscription: alerts-service
├── cosmos-sentinel-he         (Cosmos DB: sentinel-health / telemetry + alerts)
├── acs-sentinel-he            (Communication Services)
├── pg-sentinel-he             (PostgreSQL)
│   ├── DB: sentinel_users     (user-service)
│   └── DB: sentinel_calendar  (calendar-service)
├── kv-sentinel-he             (Key Vault)
└── crsentinelhe               (Container Registry)
```

### Firebase
El archivo `services/alerts-service/firebase-credentials.json` debe existir con el Service Account JSON de Firebase.

---

## 3. Configuración local

### 3.1 Login Azure

```bash
az login
az account set --subscription "<subscription-id>"
```

### 3.2 Obtener secretos de Key Vault

```bash
KV="kv-sentinel-he"
SERVICE_BUS_CONN=$(az keyvault secret show --vault-name $KV --name servicebus-connection-string --query value -o tsv)
COSMOS_ENDPOINT=$(az keyvault secret show --vault-name $KV --name cosmos-endpoint --query value -o tsv)
COSMOS_KEY=$(az keyvault secret show --vault-name $KV --name cosmos-key --query value -o tsv)
IOTHUB_CONN=$(az keyvault secret show --vault-name $KV --name iothub-eventhub-connection-string --query value -o tsv)
IOTHUB_NAME=$(az keyvault secret show --vault-name $KV --name iothub-eventhub-name --query value -o tsv)
ACS_CONN=$(az keyvault secret show --vault-name $KV --name acs-connection-string --query value -o tsv)
INTERNAL_API_KEY=$(az keyvault secret show --vault-name $KV --name internal-api-key --query value -o tsv)
JWT_SECRET=$(az keyvault secret show --vault-name $KV --name jwt-secret --query value -o tsv)
```

### 3.3 Crear archivos .env

#### user-service
```bash
cat > services/user-service/.env << EOF
DATABASE_URL=postgres://postgres:postgres@postgres:5432/sentinel_users?sslmode=disable
JWT_SECRET=${JWT_SECRET}
INTERNAL_API_KEY=${INTERNAL_API_KEY}
ACS_CONNECTION_STRING=${ACS_CONN}
ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net
RESET_PASSWORD_BASE_URL=sentinelhealth://reset-password
PORT=8080
LOG_LEVEL=debug
EOF
```

#### analytics-service
```bash
cat > services/analytics-service/.env << EOF
COSMOS_ENDPOINT=${COSMOS_ENDPOINT}
COSMOS_KEY=${COSMOS_KEY}
COSMOS_DATABASE=sentinel-health
COSMOS_TELEMETRY_CONTAINER=telemetry
COSMOS_ALERTS_CONTAINER=alerts
JWT_SECRET=${JWT_SECRET}
PORT=8080
LOG_LEVEL=debug
EOF
```

#### calendar-service
```bash
cat > services/calendar-service/.env << EOF
CALENDAR_DATABASE_URL=postgres://postgres:postgres@postgres:5432/sentinel_calendar?sslmode=disable
JWT_SECRET=${JWT_SECRET}
USER_SERVICE_URL=http://user-service:8080
USER_SERVICE_API_KEY=${INTERNAL_API_KEY}
# Notificaciones (opcionales en local — el servicio arranca sin ellas)
ACS_CONNECTION_STRING=${ACS_CONN}
ACS_SENDER_ADDRESS=DoNotReply@<your-acs-domain>.azurecomm.net
FIREBASE_CREDENTIALS_JSON=${FIREBASE_CREDS_B64}
PORT=8080
LOG_LEVEL=debug
EOF
```

#### telemetry-service
```bash
STORAGE_CONN="DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://azurite:10000/devstoreaccount1;"

cat > services/telemetry-service/.env << EOF
IOTHUB_EVENTHUB_CONNECTION_STRING=${IOTHUB_CONN}
IOTHUB_EVENTHUB_NAME=${IOTHUB_NAME}
IOTHUB_CONSUMER_GROUP=telemetry-service
AZURE_STORAGE_CONNECTION_STRING=${STORAGE_CONN}
CHECKPOINT_CONTAINER_NAME=iothub-checkpoints
SERVICE_BUS_CONNECTION_STRING=${SERVICE_BUS_CONN}
TELEMETRY_TOPIC_NAME=telemetry-received
COSMOS_ENDPOINT=${COSMOS_ENDPOINT}
COSMOS_KEY=${COSMOS_KEY}
COSMOS_DATABASE=sentinel-health
COSMOS_CONTAINER=telemetry
USER_SERVICE_URL=http://user-service:8080
USER_SERVICE_API_KEY=${INTERNAL_API_KEY}
PORT=8080
LOG_LEVEL=debug
EOF
```

#### health-rules-service
```bash
cat > services/health-rules-service/.env << EOF
SERVICE_BUS_CONNECTION_STRING=${SERVICE_BUS_CONN}
TELEMETRY_TOPIC_NAME=telemetry-received
TELEMETRY_SUBSCRIPTION_NAME=health-rules-service
ANOMALY_TOPIC_NAME=anomaly-detected
PORT=8080
LOG_LEVEL=debug
EOF
```

#### alerts-service
```bash
cat > services/alerts-service/.env << EOF
SERVICE_BUS_CONNECTION_STRING=${SERVICE_BUS_CONN}
ANOMALY_TOPIC_NAME=anomaly-detected
ANOMALY_SUBSCRIPTION_NAME=alerts-service
COSMOS_ENDPOINT=${COSMOS_ENDPOINT}
COSMOS_KEY=${COSMOS_KEY}
COSMOS_DATABASE=sentinel-health
COSMOS_ALERTS_CONTAINER=alerts
FIREBASE_CREDENTIALS_FILE=/app/firebase-credentials.json
ACS_CONNECTION_STRING=${ACS_CONN}
ACS_SENDER_ADDRESS=DoNotReply@9dc81e66-3e09-4667-a962-fe3207da1082.azurecomm.net
USER_SERVICE_URL=http://user-service:8080
USER_SERVICE_API_KEY=${INTERNAL_API_KEY}
PORT=8080
LOG_LEVEL=debug
EOF
```

---

## 4. Levantar en local

```bash
# Construir todas las imágenes (primera vez o con cambios de código)
docker compose build --no-cache

# Arrancar todo
docker compose up -d

# Verificar estado
docker compose ps

# Ver logs
docker compose logs -f
docker compose logs -f user-service   # solo un servicio
```

Estado esperado:
```
NAME                   STATUS           PORTS
azurite                running          0.0.0.0:10000->10000/tcp
postgres               running(healthy) 0.0.0.0:5432->5432/tcp
user-service           running          0.0.0.0:8080->8080/tcp
analytics-service      running          0.0.0.0:8084->8080/tcp
calendar-service       running          0.0.0.0:8085->8080/tcp
telemetry-service      running          0.0.0.0:8081->8080/tcp
health-rules-service   running          0.0.0.0:8082->8080/tcp
alerts-service         running          0.0.0.0:8083->8080/tcp
```

### Crear el primer doctor (solo una vez)

Las tablas se crean automáticamente. Solo hay que insertar el doctor:

```bash
# Paso 1 — generar hash bcrypt de la contraseña
docker run --rm golang:1.23-alpine sh -c "
cat > /tmp/h.go << 'GOEOF'
package main
import (\"fmt\"; \"golang.org/x/crypto/bcrypt\"; \"os\")
func main() {
  h, _ := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
  fmt.Print(string(h))
}
GOEOF
cd /tmp && go mod init h && go get golang.org/x/crypto && go run h.go 'MiPassword123'" 2>/dev/null

# Paso 2 — copiar el hash e insertar
HASH='$2a$10$...'   # reemplaza con el hash generado

docker exec sentinel-health-engine-be-postgres-1 psql -U postgres -d sentinel_users -c "
INSERT INTO users (id, email, password_hash, role, first_name, last_name, is_active)
VALUES (gen_random_uuid(), 'doctor@example.com', '${HASH}', 'DOCTOR', 'Diego', 'Murcia', true)
ON CONFLICT (email) DO NOTHING;"
```

---

## 5. Pruebas en local

### 5.1 Health checks

```bash
for port in 8080 8081 8082 8083 8084 8085; do
  echo -n "Puerto $port: "
  curl -s http://localhost:$port/health | jq -r '.service + " → " + .status'
done
```

### 5.2 Documentación interactiva

Abrir en el navegador:
- **http://localhost:8080/docs** — user-service
- **http://localhost:8084/docs** — analytics-service
- **http://localhost:8085/docs** — calendar-service

---

### 5.3 Autenticación y perfil

```bash
BASE="http://localhost:8080"

# Login
LOGIN=$(curl -s -X POST "$BASE/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@example.com","password":"MiPassword123"}')
ACCESS=$(echo $LOGIN | jq -r '.accessToken')
REFRESH=$(echo $LOGIN | jq -r '.refreshToken')

# Mi perfil
curl -s "$BASE/v1/users/me" -H "Authorization: Bearer $ACCESS" | jq

# Refrescar token
NEW=$(curl -s -X POST "$BASE/v1/auth/refresh" \
  -H "Content-Type: application/json" -d "{\"refreshToken\":\"$REFRESH\"}")
ACCESS=$(echo $NEW | jq -r '.accessToken')

# Cambiar contraseña
curl -s -X POST "$BASE/v1/auth/change-password" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"oldPassword":"MiPassword123","newPassword":"NuevoPass789"}' | jq

# Recuperar contraseña (siempre retorna 200)
curl -s -X POST "$BASE/v1/auth/forgot-password" \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@example.com"}' | jq
```

---

### 5.4 Pacientes y dashboard

```bash
# Crear paciente
PATIENT=$(curl -s -X POST "$BASE/v1/patients" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"firstName":"Maria","lastName":"Garcia","email":"maria@test.com"}')
PATIENT_ID=$(echo $PATIENT | jq -r '.id')

# Listar pacientes
curl -s "$BASE/v1/patients" -H "Authorization: Bearer $ACCESS" | jq

# Perfil completo (paciente + dispositivos + cuidadores en una sola llamada)
curl -s "$BASE/v1/patients/$PATIENT_ID/profile/complete" \
  -H "Authorization: Bearer $ACCESS" | jq

# Dashboard del doctor (todos los pacientes con conteos)
curl -s "$BASE/v1/doctor/dashboard" \
  -H "Authorization: Bearer $ACCESS" | jq
```

---

### 5.5 Dispositivos y cuidadores

```bash
# Registrar dispositivo (simula login móvil)
curl -s -X POST "$BASE/v1/devices/register" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"deviceIdentifier":"patient-001-android","fcmToken":"test-fcm","platform":"ANDROID","name":"Pixel 8"}' | jq

# Crear cuidador
CARETAKER_ID=$(curl -s -X POST "$BASE/v1/caretakers" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"firstName":"Carlos","lastName":"Lopez","email":"carlos@test.com"}' | jq -r '.id')

# Vincular cuidador al paciente
curl -s -X POST "$BASE/v1/patients/$PATIENT_ID/caretakers" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d "{\"caretakerId\":\"$CARETAKER_ID\"}" | jq

# Listar cuidadores del paciente
curl -s "$BASE/v1/patients/$PATIENT_ID/caretakers" \
  -H "Authorization: Bearer $ACCESS" | jq
```

---

### 5.6 Analytics — vitales e historial

```bash
ANA="http://localhost:8084"

# Historial de vitales (últimos 30 días por defecto)
curl -s "$ANA/v1/patients/$PATIENT_ID/vitals/history" \
  -H "Authorization: Bearer $ACCESS" | jq

# Último vital
curl -s "$ANA/v1/patients/$PATIENT_ID/vitals/latest" \
  -H "Authorization: Bearer $ACCESS" | jq

# Resumen estadístico (min/max/avg)
curl -s "$ANA/v1/patients/$PATIENT_ID/vitals/summary?from=2024-01-01&to=2024-12-31" \
  -H "Authorization: Bearer $ACCESS" | jq

# Historial de alertas
curl -s "$ANA/v1/patients/$PATIENT_ID/alerts/history" \
  -H "Authorization: Bearer $ACCESS" | jq

# Solo alertas críticas
curl -s "$ANA/v1/patients/$PATIENT_ID/alerts/history?severity=CRITICAL" \
  -H "Authorization: Bearer $ACCESS" | jq

# Estadísticas de alertas
curl -s "$ANA/v1/patients/$PATIENT_ID/alerts/stats" \
  -H "Authorization: Bearer $ACCESS" | jq

# Generar reporte PDF
curl -s -X POST "$ANA/v1/patients/$PATIENT_ID/reports/generate" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"from":"2024-01-01","to":"2024-12-31"}' \
  --output "reporte-$PATIENT_ID.pdf"
echo "PDF guardado: reporte-$PATIENT_ID.pdf"
```

---

### 5.7 Calendar — citas médicas

```bash
CAL="http://localhost:8085"

# Crear cita médica (7 días desde ahora)
APPT_ID=$(curl -s -X POST "$CAL/v1/patients/$PATIENT_ID/appointments" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{
    "title":"Revisión mensual",
    "scheduledAt":"2025-06-01T10:00:00Z",
    "location":"Consultorio 3",
    "notes":"Traer resultados de laboratorio"
  }' | jq -r '.id')

# Listar citas
curl -s "$CAL/v1/patients/$PATIENT_ID/appointments" \
  -H "Authorization: Bearer $ACCESS" | jq

# Completar cita
curl -s -X PUT "$CAL/v1/patients/$PATIENT_ID/appointments/$APPT_ID" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"status":"COMPLETED","notes":"Revisión completada sin novedad"}' | jq

# Cancelar cita
curl -s -X DELETE "$CAL/v1/patients/$PATIENT_ID/appointments/$APPT_ID" \
  -H "Authorization: Bearer $ACCESS" | jq
```

---

### 5.8 Calendar — medicamentos

```bash
# Registrar medicamento diario
MED_ID=$(curl -s -X POST "$CAL/v1/patients/$PATIENT_ID/medications" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{
    "name":"Losartan","dosage":"50mg","frequency":"DAILY",
    "scheduledTimes":["08:00"],"startDate":"2024-01-01",
    "notes":"Tomar con el desayuno"
  }' | jq -r '.id')

# Medicamento dos veces al día
curl -s -X POST "$CAL/v1/patients/$PATIENT_ID/medications" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{
    "name":"Metformina","dosage":"500mg","frequency":"TWICE_DAILY",
    "scheduledTimes":["08:00","20:00"],"startDate":"2024-01-01"
  }' | jq

# Listar medicamentos activos
curl -s "$CAL/v1/patients/$PATIENT_ID/medications?active=true" \
  -H "Authorization: Bearer $ACCESS" | jq

# Dar de baja medicamento (soft delete)
curl -s -X DELETE "$CAL/v1/patients/$PATIENT_ID/medications/$MED_ID" \
  -H "Authorization: Bearer $ACCESS" | jq
```

---

### 5.9 Calendar — recordatorios

```bash
# Crear recordatorio diario
REM_ID=$(curl -s -X POST "$CAL/v1/patients/$PATIENT_ID/reminders" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{
    "title":"Control de presión",
    "message":"Recuerda medir tu presión arterial antes del desayuno",
    "reminderAt":"2025-05-01T07:00:00Z",
    "recurrence":"DAILY"
  }' | jq -r '.id')

# Recordatorios de hoy
curl -s "$CAL/v1/patients/$PATIENT_ID/reminders/today" \
  -H "Authorization: Bearer $ACCESS" | jq

# Todos los recordatorios
curl -s "$CAL/v1/patients/$PATIENT_ID/reminders" \
  -H "Authorization: Bearer $ACCESS" | jq

# Cancelar recordatorio
curl -s -X DELETE "$CAL/v1/patients/$PATIENT_ID/reminders/$REM_ID" \
  -H "Authorization: Bearer $ACCESS" | jq
```

---

### 5.10 Pipeline IoT completo

```bash
# Simular telemetría con valores anómalos
az extension add --name azure-iot 2>/dev/null
az iot device simulate \
  --hub-name iothub-sentinel-he \
  --device-id patient-001-android \
  --data '{"heartRate":135,"spo2":82}' \
  --msg-count 1

# Ver el flujo completo en logs
docker compose logs -f telemetry-service health-rules-service alerts-service
```

---

## 6. Despliegue en Azure

### 6.1 Prerequisitos (solo primera vez)

```bash
# 1. Aprovisionar PostgreSQL para user-service (si no se hizo antes)
bash scripts/provision-user-service.sh

# 2. Crear base de datos sentinel_calendar para calendar-service
bash scripts/provision-calendar-service.sh
```

### 6.2 Desplegar todos los servicios

```bash
bash scripts/deploy-all.sh
```

Construye y despliega los 6 servicios en orden correcto. Tarda ~5-10 min.

### 6.3 Crear el primer doctor en cloud

```bash
bash scripts/seed-first-doctor.sh \
  --first-name "Diego" \
  --last-name "Murcia" \
  --email "doctor@tudominio.com" \
  --password "TuPasswordSeguro123"
```

---

## 7. Pruebas en Azure

### 7.1 Obtener URLs

```bash
RG="rg-sentinel-health-engine"

for SVC in user-service analytics-service calendar-service; do
  URL="https://$(az containerapp show --name $SVC --resource-group $RG \
    --query properties.configuration.ingress.fqdn -o tsv)"
  echo "$SVC: $URL"
done
```

### 7.2 Health checks

```bash
# Reemplaza con tus FQDNs reales
curl -s "https://<user-fqdn>/health" | jq
curl -s "https://<analytics-fqdn>/health" | jq
curl -s "https://<calendar-fqdn>/health" | jq
```

### 7.3 Docs en cloud

```
https://<user-fqdn>/docs
https://<analytics-fqdn>/docs
https://<calendar-fqdn>/docs
```

### 7.4 Pruebas funcionales en cloud

Usa los mismos comandos de la sección 5 reemplazando:
- `http://localhost:8080` → `https://<user-fqdn>`
- `http://localhost:8084` → `https://<analytics-fqdn>`
- `http://localhost:8085` → `https://<calendar-fqdn>`

### 7.5 Ver logs en cloud

```bash
for SVC in user-service analytics-service calendar-service alerts-service; do
  echo "=== $SVC ==="
  az containerapp logs show --name $SVC \
    --resource-group rg-sentinel-health-engine --tail 20
done
```

---

## 8. Referencia de puertos y URLs

| Servicio | Puerto local | Swagger UI local |
|---|---|---|
| user-service | `8080` | http://localhost:8080/docs |
| telemetry-service | `8081` | http://localhost:8081/health |
| health-rules-service | `8082` | http://localhost:8082/health |
| alerts-service | `8083` | http://localhost:8083/health |
| analytics-service | `8084` | http://localhost:8084/docs |
| calendar-service | `8085` | http://localhost:8085/docs |
| PostgreSQL | `5432` | `psql -h localhost -U postgres` |
| Azurite | `10000` | emulador Azure Storage |

---

## 9. Variables de entorno

### user-service
| Variable | Descripción | Req |
|---|---|---|
| `DATABASE_URL` | PostgreSQL (sentinel_users) | ✅ |
| `JWT_SECRET` | Secreto HMAC-SHA256 para JWT | ✅ |
| `INTERNAL_API_KEY` | API key service-to-service | ✅ |
| `ACS_CONNECTION_STRING` | Azure Communication Services | ⚠️ |
| `ACS_SENDER_ADDRESS` | Email remitente | ⚠️ |
| `RESET_PASSWORD_BASE_URL` | Base URL del deep link de reset | opcional |

### analytics-service
| Variable | Descripción | Req |
|---|---|---|
| `COSMOS_ENDPOINT` | URL de Cosmos DB | ✅ |
| `COSMOS_KEY` | Primary key Cosmos DB | ✅ |
| `COSMOS_DATABASE` | Nombre de la base de datos (`sentinel-health`) | ✅ |
| `COSMOS_TELEMETRY_CONTAINER` | Contenedor de telemetría (`telemetry`) | ✅ |
| `COSMOS_ALERTS_CONTAINER` | Contenedor de alertas (`alerts`) | ✅ |
| `JWT_SECRET` | Mismo que user-service | ✅ |

### calendar-service
| Variable | Descripción | Req |
|---|---|---|
| `CALENDAR_DATABASE_URL` | PostgreSQL (sentinel_calendar) | ✅ |
| `JWT_SECRET` | Mismo que user-service | ✅ |
| `USER_SERVICE_URL` | URL base del user-service | ✅ |
| `USER_SERVICE_API_KEY` | Mismo que INTERNAL_API_KEY | ✅ |
| `ACS_CONNECTION_STRING` + `ACS_SENDER_ADDRESS` | Email (recordatorios y citas) | ⚠️ |
| `FIREBASE_CREDENTIALS_FILE` o `FIREBASE_CREDENTIALS_JSON` | Push notifications (recordatorios y citas) | ⚠️ |

### telemetry-service
| Variable | Descripción | Req |
|---|---|---|
| `IOTHUB_EVENTHUB_CONNECTION_STRING` | Endpoint Event Hub IoT Hub | ✅ |
| `IOTHUB_EVENTHUB_NAME` | Nombre interno del Event Hub | ✅ |
| `IOTHUB_CONSUMER_GROUP` | Consumer group | ✅ |
| `AZURE_STORAGE_CONNECTION_STRING` | Checkpoints (Azurite en local) | ✅ |
| `SERVICE_BUS_CONNECTION_STRING` | Namespace Service Bus | ✅ |
| `TELEMETRY_TOPIC_NAME` | Topic destino | ✅ |
| `COSMOS_*` | Cosmos DB (4 vars) | ✅ |
| `USER_SERVICE_URL` / `USER_SERVICE_API_KEY` | Validación de dispositivos | ✅ |

### health-rules-service
| Variable | Descripción | Req |
|---|---|---|
| `SERVICE_BUS_CONNECTION_STRING` | Namespace Service Bus | ✅ |
| `TELEMETRY_TOPIC_NAME` + `TELEMETRY_SUBSCRIPTION_NAME` | Topic entrada | ✅ |
| `ANOMALY_TOPIC_NAME` | Topic salida | ✅ |

### alerts-service
| Variable | Descripción | Req |
|---|---|---|
| `SERVICE_BUS_CONNECTION_STRING` | Namespace Service Bus | ✅ |
| `ANOMALY_TOPIC_NAME` + `ANOMALY_SUBSCRIPTION_NAME` | Topic entrada | ✅ |
| `COSMOS_*` | Cosmos DB (4 vars) | ✅ |
| `FIREBASE_CREDENTIALS_FILE` o `FIREBASE_CREDENTIALS_JSON` | Firebase | ✅ |
| `ACS_CONNECTION_STRING` + `ACS_SENDER_ADDRESS` | Email | ⚠️ |
| `USER_SERVICE_URL` / `USER_SERVICE_API_KEY` | Obtener contactos del paciente | ✅ |
