# Sentinel Health Engine — Testing Guide

This guide explains how to exercise the complete telemetry-to-alert pipeline end to end.
Part A covers testing with a real mobile device and smartwatch. Part B covers fully
manual testing using only curl commands and the Azure CLI — no physical hardware required.

---

## Table of Contents

1. [Before You Start](#1-before-you-start)
2. [Part A — Testing with a Mobile Device and Smartwatch](#2-part-a--testing-with-a-mobile-device-and-smartwatch)
3. [Part B — Testing Without a Mobile Device](#3-part-b--testing-without-a-mobile-device)
4. [Checking Logs](#4-checking-logs)
5. [Verifying Cosmos DB Entries](#5-verifying-cosmos-db-entries)
6. [End-to-End Verification Checklist](#6-end-to-end-verification-checklist)

---

## 1. Before You Start

### Required

- All four services must be running (at least one replica each). See
  `scripts/powerup-services.sh` in the Deployment Guide.
- A doctor account must exist in PostgreSQL. See Section 3 of the Deployment Guide.
- The `USER_SERVICE_URL` environment variable must be set in your shell:

```bash
USER_SERVICE_URL="https://$(az containerapp show \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --query properties.configuration.ingress.fqdn -o tsv)"

echo "$USER_SERVICE_URL"
```

### Obtain a Doctor JWT

All protected API calls require a valid JWT in the `Authorization` header.
Log in as the seeded doctor:

```bash
DOCTOR_TOKEN=$(curl -s -X POST "$USER_SERVICE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"doctor@hospital.com","password":"YourPassword123!"}' \
  | jq -r .accessToken)

echo "$DOCTOR_TOKEN"   # should print a long JWT string, not "null"
```

### Retrieve the Internal API Key

Some verification calls use the service-to-service API key:

```bash
INTERNAL_API_KEY=$(az keyvault secret show \
  --vault-name kv-sentinel-he \
  --name internal-api-key --query value -o tsv)
```

### IoT Hub Name

For Part B you will need the IoT Hub name. Retrieve it:

```bash
IOT_HUB_NAME=$(az iot hub list \
  --resource-group rg-sentinel-health-engine \
  --query "[0].name" -o tsv)
echo "$IOT_HUB_NAME"
```

---

## 2. Part A — Testing with a Mobile Device and Smartwatch

### Prerequisites

- A physical iOS or Android device with the Sentinel mobile app installed.
- A compatible smartwatch paired to the mobile app.
- The mobile app must be configured to point to `$USER_SERVICE_URL`.

### Step A1 — Doctor Creates a Patient

The doctor calls the API to create a patient record. The system automatically
sends a welcome email with a temporary password via Azure Communication Services.

```bash
curl -s -X POST "$USER_SERVICE_URL/v1/patients" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "John",
    "lastName":  "Doe",
    "email":     "patient@example.com"
  }' | jq .
```

Expected response (HTTP 201):

```json
{
  "id": "a1b2c3d4-...",
  "email": "patient@example.com",
  "role": "PATIENT",
  "firstName": "John",
  "lastName": "Doe",
  "fullName": "John Doe",
  "isActive": true,
  "createdAt": "2026-04-01T00:00:00Z"
}
```

Save the patient ID:

```bash
PATIENT_ID="a1b2c3d4-..."
```

Verify the patient received the welcome email in their inbox before proceeding.

### Step A2 — Patient Logs In and Registers the Device

On the mobile app, the patient logs in with the credentials from the welcome email.
On successful login, the app calls `POST /v1/devices/register` automatically with
the device's FCM token and platform information.

To verify the device was registered, the doctor can list the patient's devices
(via the internal endpoint used by telemetry-service):

```bash
curl -s "$USER_SERVICE_URL/v1/internal/devices/mobile-gateway-01" \
  -H "X-Internal-API-Key: $INTERNAL_API_KEY" | jq .
```

> **Note:** `mobile-gateway-01` is the `deviceIdentifier` the mobile app sends
> when registering. The actual identifier depends on your app's registration logic.

### Step A3 — (Optional) Link a Caretaker

A caretaker must already have a user account with `role = CARETAKER` in the database.
After creating the caretaker record, link them to the patient:

```bash
CARETAKER_ID="<uuid-of-caretaker-user>"

curl -s -X POST "$USER_SERVICE_URL/v1/patients/$PATIENT_ID/caretakers" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"caretakerId\": \"$CARETAKER_ID\"}" | jq .
```

Expected response (HTTP 201):

```json
{"message": "caretaker linked successfully"}
```

Verify the link:

```bash
curl -s "$USER_SERVICE_URL/v1/patients/$PATIENT_ID/caretakers" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" | jq .
```

### Step A4 — Simulate Critical Vital Signs via Smartwatch

From the smartwatch, trigger readings that will exceed the alert thresholds defined
in health-rules-service. Typical thresholds:

| Metric | Alert Condition |
|--------|----------------|
| Heart Rate | > 140 bpm or < 40 bpm |
| SpO2 | < 90% |

Wear the device and trigger the abnormal readings (or use the app's test/demo mode
if one is available).

### Step A5 — Verify the Full Pipeline

The expected flow is:

```
Smartwatch → Mobile App (IoT gateway)
  → IoT Hub
    → telemetry-service (validates device via user-service → persists to Cosmos DB → publishes to Service Bus)
      → health-rules-service (detects anomaly → publishes to Service Bus)
        → alerts-service (resolves contacts via user-service → sends FCM push + ACS email)
```

Follow the logs of each service in order (see Section 4) to confirm each step fires.
The key log lines to look for are documented in Section 4.

### Step A6 — Verify Push Notification on Device

The patient's and caretaker's mobile devices should receive a Firebase Cloud Messaging
push notification within 5–15 seconds of the anomalous reading being transmitted.

### Step A7 — Verify Email

Check the inbox of the patient and any linked caretakers for an alert email sent from
the ACS sender address configured in `ACS_SENDER_ADDRESS`.

---

## 3. Part B — Testing Without a Mobile Device

This section uses only curl commands and the Azure CLI. No physical hardware is needed.

### Step B1 — Register Test Data via API

#### B1a — Create a Patient (as Doctor)

```bash
PATIENT=$(curl -s -X POST "$USER_SERVICE_URL/v1/patients" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Test",
    "lastName":  "Patient",
    "email":     "testpatient@example.com"
  }')

echo "$PATIENT" | jq .
PATIENT_ID=$(echo "$PATIENT" | jq -r .id)
echo "Patient ID: $PATIENT_ID"
```

#### B1b — Create a Caretaker

Because there is no public registration endpoint, insert the caretaker record directly
in PostgreSQL (the same way the first doctor was seeded). Use `role = 'CARETAKER'` and
a bcrypt-hashed password.

```sql
INSERT INTO users (id, email, password_hash, role, first_name, last_name, is_active, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'caretaker@example.com',
    '$2a$12$REPLACE_WITH_ACTUAL_BCRYPT_HASH',
    'CARETAKER',
    'Test',
    'Caretaker',
    true,
    NOW(),
    NOW()
);
```

Then retrieve the caretaker's UUID:

```sql
SELECT id FROM users WHERE email = 'caretaker@example.com';
```

```bash
CARETAKER_ID="<uuid-from-above>"
```

#### B1c — Link the Caretaker to the Patient

```bash
curl -s -X POST "$USER_SERVICE_URL/v1/patients/$PATIENT_ID/caretakers" \
  -H "Authorization: Bearer $DOCTOR_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"caretakerId\": \"$CARETAKER_ID\"}" | jq .
```

#### B1d — Register a Test Device with a Fake (or Real) FCM Token

First, obtain a patient JWT:

```bash
PATIENT_TOKEN=$(curl -s -X POST "$USER_SERVICE_URL/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"testpatient@example.com","password":"<patient-temp-password>"}' \
  | jq -r .accessToken)
```

Then register the device. Use a real FCM registration token from Firebase Console if
you want actual push notifications; otherwise use a placeholder:

```bash
curl -s -X POST "$USER_SERVICE_URL/v1/devices/register" \
  -H "Authorization: Bearer $PATIENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "deviceIdentifier": "mobile-gateway-01",
    "fcmToken":         "fCm_TEST_TOKEN_REPLACE_WITH_REAL_ONE",
    "platform":         "android",
    "name":             "Test Device"
  }' | jq .
```

Expected response (HTTP 200):

```json
{
  "id": "...",
  "deviceIdentifier": "mobile-gateway-01",
  "platform": "android",
  "name": "Test Device",
  "isActive": true,
  "createdAt": "2026-04-01T00:00:00Z"
}
```

Verify the device is visible to telemetry-service:

```bash
curl -s "$USER_SERVICE_URL/v1/internal/devices/mobile-gateway-01" \
  -H "X-Internal-API-Key: $INTERNAL_API_KEY" | jq .
```

Expected response:

```json
{
  "patientId": "<PATIENT_ID>",
  "userId":    "<PATIENT_USER_ID>",
  "deviceId":  "<DEVICE_UUID>",
  "isActive":  true
}
```

Verify contact resolution works for alerts-service:

```bash
curl -s "$USER_SERVICE_URL/v1/internal/patients/$PATIENT_ID/contacts" \
  -H "X-Internal-API-Key: $INTERNAL_API_KEY" | jq .
```

Expected response (patient + caretaker):

```json
{
  "contacts": [
    {"email": "testpatient@example.com", "fcmToken": "fCm_TEST_TOKEN_REPLACE_WITH_REAL_ONE"},
    {"email": "caretaker@example.com",   "fcmToken": ""}
  ]
}
```

### Step B2 — Send a Simulated IoT Message

Use the Azure CLI `az iot` extension to inject a device-to-cloud message directly
into IoT Hub, bypassing the need for a physical device:

```bash
az iot device send-d2c-message \
  --hub-name "$IOT_HUB_NAME" \
  --device-id mobile-gateway-01 \
  --data '{
    "deviceId":  "mobile-gateway-01",
    "heartRate": 145,
    "spO2":      88,
    "timestamp": "2026-04-01T00:00:00Z"
  }'
```

> **Important:** The `device-id` here must exactly match the `deviceIdentifier`
> registered in user-service (`mobile-gateway-01` in this example).
> The IoT device `mobile-gateway-01` must also exist in your IoT Hub. If it does
> not, create it first:
>
> ```bash
> az iot hub device-identity create \
>   --hub-name "$IOT_HUB_NAME" \
>   --device-id mobile-gateway-01
> ```

The values `heartRate: 145` and `spO2: 88` should exceed the anomaly thresholds in
health-rules-service and trigger an alert.

### Step B3 — Monitor Logs for Each Service

Open four terminal windows and tail the logs of each service simultaneously
(see Section 4 for the full log commands).

Expected log sequence after sending the IoT message:

**telemetry-service:**
```
received telemetry message  deviceId=mobile-gateway-01
device validated            patientId=<PATIENT_ID>
telemetry persisted         cosmosId=...
event published             topic=telemetry-received
```

**health-rules-service:**
```
telemetry event received    patientId=<PATIENT_ID>
anomaly detected            heartRate=145 spO2=88
anomaly event published     topic=anomaly-detected
```

**alerts-service:**
```
anomaly event received      patientId=<PATIENT_ID>
contacts resolved           count=2
push notification sent      fcmToken=fCm_TEST_TOKEN...
email sent                  to=testpatient@example.com
email sent                  to=caretaker@example.com
alert persisted             cosmosId=...
```

### Step B4 — Verify Cosmos DB Entries

See Section 5 for detailed Cosmos DB verification steps.

### Step B5 — Verify Notifications Received

- **Push notification:** If you used a real FCM token from Firebase Console, check the
  target device. If you used a placeholder token, the push will fail silently but the
  log will confirm the attempt.
- **Email:** Check the inboxes of `testpatient@example.com` and
  `caretaker@example.com`. If using test addresses, check Azure Communication Services
  message logs instead (see below).

Check ACS email send status:

```bash
# List recent ACS email send operations (requires az communication extension)
az communication email status list \
  --connection-string "$(az keyvault secret show \
    --vault-name kv-sentinel-he \
    --name acs-connection-string --query value -o tsv)"
```

---

## 4. Checking Logs

### Real-Time Log Streaming

Use the following command to stream logs for any service. Replace `<service-name>`
with `user-service`, `telemetry-service`, `health-rules-service`, or `alerts-service`.

```bash
az containerapp logs show \
  --name <service-name> \
  --resource-group rg-sentinel-health-engine \
  --follow
```

### Stream All Four Services in Parallel

Open four separate terminal windows and run one command per window:

```bash
# Terminal 1
az containerapp logs show \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --follow

# Terminal 2
az containerapp logs show \
  --name telemetry-service \
  --resource-group rg-sentinel-health-engine \
  --follow

# Terminal 3
az containerapp logs show \
  --name health-rules-service \
  --resource-group rg-sentinel-health-engine \
  --follow

# Terminal 4
az containerapp logs show \
  --name alerts-service \
  --resource-group rg-sentinel-health-engine \
  --follow
```

### View Recent Logs Without Following

```bash
az containerapp logs show \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --tail 100
```

### Filtering by Revision

If multiple revisions are deployed, filter to the active one:

```bash
REVISION=$(az containerapp revision list \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --query "[?properties.active].name | [0]" -o tsv)

az containerapp logs show \
  --name user-service \
  --resource-group rg-sentinel-health-engine \
  --revision "$REVISION" \
  --follow
```

### Log via Azure Monitor (if configured)

```bash
az monitor log-analytics query \
  --workspace <log-analytics-workspace-id> \
  --analytics-query "ContainerAppConsoleLogs_CL
    | where ContainerAppName_s == 'user-service'
    | order by TimeGenerated desc
    | take 50" \
  --output table
```

---

## 5. Verifying Cosmos DB Entries

### Retrieve Cosmos DB Credentials

```bash
COSMOS_ENDPOINT=$(az keyvault secret show \
  --vault-name kv-sentinel-he \
  --name cosmos-endpoint --query value -o tsv)

COSMOS_KEY=$(az keyvault secret show \
  --vault-name kv-sentinel-he \
  --name cosmos-key --query value -o tsv)
```

### Query via Azure Portal

1. Go to the Azure Portal → your Cosmos DB account → **Data Explorer**.
2. Navigate to database `sentinel-health`.
3. Check container `telemetry` for the persisted reading.
4. Check container `alerts` for the generated alert document.

### Query via Azure CLI

```bash
# List recent telemetry documents
az cosmosdb sql query \
  --account-name <cosmos-account-name> \
  --resource-group rg-sentinel-health-engine \
  --database-name sentinel-health \
  --container-name telemetry \
  --query-text "SELECT TOP 5 * FROM c ORDER BY c._ts DESC"

# List recent alert documents
az cosmosdb sql query \
  --account-name <cosmos-account-name> \
  --resource-group rg-sentinel-health-engine \
  --database-name sentinel-health \
  --container-name alerts \
  --query-text "SELECT TOP 5 * FROM c ORDER BY c._ts DESC"
```

### Expected Telemetry Document Schema

```json
{
  "id": "<uuid>",
  "deviceId": "mobile-gateway-01",
  "patientId": "<PATIENT_ID>",
  "heartRate": 145,
  "spO2": 88,
  "timestamp": "2026-04-01T00:00:00Z",
  "_ts": 1743465600
}
```

### Expected Alert Document Schema

```json
{
  "id": "<uuid>",
  "patientId": "<PATIENT_ID>",
  "alertType": "CRITICAL_VITALS",
  "vitals": {
    "heartRate": 145,
    "spO2": 88
  },
  "notificationsSent": [
    {"channel": "push",  "recipient": "fCm_TEST_TOKEN...", "status": "sent"},
    {"channel": "email", "recipient": "testpatient@example.com", "status": "sent"},
    {"channel": "email", "recipient": "caretaker@example.com",   "status": "sent"}
  ],
  "createdAt": "2026-04-01T00:00:05Z",
  "_ts": 1743465605
}
```

---

## 6. End-to-End Verification Checklist

Use this checklist after every significant deployment or code change.

### Infrastructure

- [ ] `GET /health` returns HTTP 200 on all four services
- [ ] PostgreSQL is reachable from user-service (check user-service logs for "postgres connected")
- [ ] Cosmos DB is reachable from telemetry-service and alerts-service (no connection errors in logs)
- [ ] Service Bus topic `telemetry-received` exists and telemetry-service can publish to it
- [ ] Service Bus topic `anomaly-detected` exists and health-rules-service can publish to it

### user-service

- [ ] Doctor can log in: `POST /v1/auth/login` returns a token pair
- [ ] `GET /v1/users/me` returns the doctor's profile with `role: "DOCTOR"`
- [ ] Doctor can create a patient: `POST /v1/patients` returns HTTP 201
- [ ] Patient welcome email is received
- [ ] Patient can log in and register a device: `POST /v1/devices/register` returns HTTP 200
- [ ] Caretaker can be linked to patient: `POST /v1/patients/:id/caretakers` returns HTTP 201
- [ ] Internal device validation returns the correct `patientId`
- [ ] Internal contact resolution returns the patient and all linked caretakers

### Full Pipeline

- [ ] IoT message with normal vitals does NOT produce an alert
- [ ] IoT message with `heartRate > 140` produces an alert
- [ ] IoT message with `spO2 < 90` produces an alert
- [ ] telemetry-service logs show device validated via user-service
- [ ] telemetry-service persists document to Cosmos DB `telemetry` container
- [ ] health-rules-service logs show anomaly detected and event published
- [ ] alerts-service logs show contacts resolved via user-service
- [ ] alerts-service logs show push notification attempt
- [ ] alerts-service logs show email send attempt
- [ ] Alert document persisted to Cosmos DB `alerts` container
- [ ] (If real FCM token) Push notification received on device
- [ ] (If real email) Alert email received in inbox

### Token Refresh

- [ ] `POST /v1/auth/refresh` with a valid refresh token returns a new access token
- [ ] Expired or invalid tokens return HTTP 401

### Password Change

- [ ] `POST /v1/auth/change-password` with correct old password returns HTTP 200
- [ ] Subsequent login with new password succeeds
- [ ] Login with old password returns HTTP 401
