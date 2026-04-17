// Package deviceregistry provides a DeviceRegistry implementation backed by the user-service HTTP API.
package deviceregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// cacheEntry holds a cached device lookup result.
type cacheEntry struct {
	patientID string
	expiresAt time.Time
}

// UserServiceRegistry implements domain.DeviceRegistry by calling the user-service.
// Results are cached in memory for cacheTTL to reduce API calls on high-throughput telemetry.
type UserServiceRegistry struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      sync.Map // map[deviceID]cacheEntry
	cacheTTL   time.Duration
}

// NewUserServiceRegistry creates a registry that calls the user-service.
// Required env vars: USER_SERVICE_URL, USER_SERVICE_API_KEY
func NewUserServiceRegistry() (*UserServiceRegistry, error) {
	baseURL := os.Getenv("USER_SERVICE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL env var is required")
	}
	apiKey := os.Getenv("USER_SERVICE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("USER_SERVICE_API_KEY env var is required")
	}

	return &UserServiceRegistry{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cacheTTL:   5 * time.Minute,
	}, nil
}

// IsAuthorized checks if a device is registered in user-service and returns the patient ID.
func (r *UserServiceRegistry) IsAuthorized(ctx context.Context, deviceID string) (string, bool, error) {
	// Check in-memory cache first.
	if entry, ok := r.cache.Load(deviceID); ok {
		cached := entry.(cacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.patientID, true, nil
		}
		r.cache.Delete(deviceID)
	}

	patientID, err := r.fetchFromUserService(ctx, deviceID)
	if err != nil {
		return "", false, nil // treat lookup failure as unauthorized (safe default)
	}

	r.cache.Store(deviceID, cacheEntry{
		patientID: patientID,
		expiresAt: time.Now().Add(r.cacheTTL),
	})

	return patientID, true, nil
}

func (r *UserServiceRegistry) fetchFromUserService(ctx context.Context, deviceID string) (string, error) {
	url := fmt.Sprintf("%s/v1/internal/devices/%s", r.baseURL, deviceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Internal-API-Key", r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call user-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("device not found")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user-service returned %d", resp.StatusCode)
	}

	var result struct {
		PatientID string `json:"patientId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.PatientID == "" {
		return "", fmt.Errorf("empty patientId in response")
	}

	return result.PatientID, nil
}
