// Package iothub provides a client for the Azure IoT Hub Device Registry REST API.
// It uses only the Go standard library — no Azure SDK is required.
package iothub

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Registry manages device identities in Azure IoT Hub using the REST API.
type Registry struct {
	hostname   string
	keyName    string
	key        []byte
	httpClient *http.Client
}

// NewRegistryFromConnectionString parses an IoT Hub connection string and
// returns a Registry client.
//
// Connection string format:
//
//	HostName=xxx.azure-devices.net;SharedAccessKeyName=xxx;SharedAccessKey=xxx
func NewRegistryFromConnectionString(connStr string) (*Registry, error) {
	params := make(map[string]string)
	for _, part := range strings.Split(connStr, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	hostname := params["HostName"]
	keyName := params["SharedAccessKeyName"]
	keyB64 := params["SharedAccessKey"]
	if hostname == "" || keyName == "" || keyB64 == "" {
		return nil, fmt.Errorf("invalid IoT Hub connection string: missing HostName, SharedAccessKeyName or SharedAccessKey")
	}
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode shared access key: %w", err)
	}
	return &Registry{
		hostname:   hostname,
		keyName:    keyName,
		key:        key,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// EnsureDevice creates the device identity in IoT Hub if it doesn't already exist.
// Idempotent: if the device already exists (HTTP 409), it is treated as success.
func (r *Registry) EnsureDevice(ctx context.Context, deviceID string) error {
	endpoint := fmt.Sprintf(
		"https://%s/devices/%s?api-version=2021-04-12",
		r.hostname, url.PathEscape(deviceID),
	)

	body, _ := json.Marshal(map[string]string{"deviceId": deviceID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", r.sasToken(3600))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call IoT Hub registry: %w", err)
	}
	defer resp.Body.Close()

	// 200 = updated existing, 201 = created new, 409 = already exists (idempotent)
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusConflict:
		return nil
	default:
		return fmt.Errorf("IoT Hub registry returned unexpected status %d for device %q", resp.StatusCode, deviceID)
	}
}

// sasToken generates an IoT Hub SAS token valid for ttlSeconds seconds.
// The resource URI is scoped to the entire hub (service-level token).
func (r *Registry) sasToken(ttlSeconds int64) string {
	expiry := strconv.FormatInt(time.Now().Unix()+ttlSeconds, 10)
	resourceURI := url.QueryEscape(r.hostname)
	stringToSign := resourceURI + "\n" + expiry

	mac := hmac.New(sha256.New, r.key)
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf(
		"SharedAccessSignature sr=%s&sig=%s&se=%s&skn=%s",
		resourceURI, url.QueryEscape(sig), expiry, r.keyName,
	)
}
