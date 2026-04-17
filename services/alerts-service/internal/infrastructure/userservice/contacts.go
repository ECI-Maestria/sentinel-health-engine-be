// Package userservice provides a ContactResolver backed by the user-service HTTP API.
package userservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Contact holds notification coordinates for one person.
type Contact struct {
	Email    string
	FCMToken string
}

// ContactResolver calls the user-service to fetch contacts for a patient.
type ContactResolver struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewContactResolver creates a resolver that calls the user-service.
// Required env vars: USER_SERVICE_URL, USER_SERVICE_API_KEY
func NewContactResolver() (*ContactResolver, error) {
	baseURL := os.Getenv("USER_SERVICE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL env var is required")
	}
	apiKey := os.Getenv("USER_SERVICE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("USER_SERVICE_API_KEY env var is required")
	}

	return &ContactResolver{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// GetContacts returns all notification contacts for the given patient ID.
func (r *ContactResolver) GetContacts(ctx context.Context, patientID string) ([]Contact, error) {
	url := fmt.Sprintf("%s/v1/internal/patients/%s/contacts", r.baseURL, patientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Internal-API-Key", r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call user-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("patient not found: %s", patientID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user-service returned %d", resp.StatusCode)
	}

	var result struct {
		Contacts []struct {
			Email    string `json:"email"`
			FCMToken string `json:"fcmToken"`
		} `json:"contacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	contacts := make([]Contact, 0, len(result.Contacts))
	for _, c := range result.Contacts {
		contacts = append(contacts, Contact{Email: c.Email, FCMToken: c.FCMToken})
	}
	return contacts, nil
}
