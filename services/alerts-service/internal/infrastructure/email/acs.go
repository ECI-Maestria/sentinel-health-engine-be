// Package email implements email delivery via Azure Communication Services REST API.
package email

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ACSEmailSender sends emails via Azure Communication Services.
type ACSEmailSender struct {
	endpoint      string
	accessKey     []byte // decoded bytes, used for HMAC signing
	senderAddress string
	httpClient    *http.Client
	logger        *zap.Logger
}

// NewACSEmailSender builds the sender from ACS_CONNECTION_STRING env var.
// Connection string format: endpoint=https://...;accesskey=Base64Key==
func NewACSEmailSender(logger *zap.Logger) (*ACSEmailSender, error) {
	connStr := os.Getenv("ACS_CONNECTION_STRING")
	if connStr == "" {
		return nil, fmt.Errorf("ACS_CONNECTION_STRING is required")
	}
	endpoint, accessKeyB64, err := parseConnStr(connStr)
	if err != nil {
		return nil, err
	}

	// The access key in the connection string is base64-encoded; decode it for HMAC signing.
	keyBytes, err := base64.StdEncoding.DecodeString(accessKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode ACS access key: %w", err)
	}

	senderAddr := os.Getenv("ACS_SENDER_ADDRESS")
	if senderAddr == "" {
		return nil, fmt.Errorf("ACS_SENDER_ADDRESS is required")
	}

	return &ACSEmailSender{
		endpoint:      strings.TrimRight(endpoint, "/"),
		accessKey:     keyBytes,
		senderAddress: senderAddr,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
		logger:        logger,
	}, nil
}

// SendAlert sends an HTML alert email to recipientEmail.
func (s *ACSEmailSender) SendAlert(ctx context.Context, recipientEmail, subject, htmlBody string) error {
	if recipientEmail == "" {
		s.logger.Warn("skipping email: recipient is empty")
		return nil
	}

	payload := map[string]interface{}{
		"senderAddress": s.senderAddress,
		"recipients": map[string]interface{}{
			"to": []map[string]string{{"address": recipientEmail}},
		},
		"content": map[string]string{
			"subject": subject,
			"html":    htmlBody,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}

	url := fmt.Sprintf("%s/emails:send?api-version=2023-03-31", s.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build ACS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// ACS requires HMAC-SHA256 authentication — NOT Bearer token.
	if err := s.signRequest(req, body); err != nil {
		return fmt.Errorf("sign ACS request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ACS HTTP call failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ACS returned %d: %s", resp.StatusCode, string(respBody))
	}

	s.logger.Info("email sent via ACS",
		zap.String("to", recipientEmail),
		zap.String("subject", subject),
	)
	return nil
}

// signRequest adds the HMAC-SHA256 authentication headers required by ACS REST API.
// Spec: https://learn.microsoft.com/azure/communication-services/concepts/authentication
func (s *ACSEmailSender) signRequest(req *http.Request, body []byte) error {
	// 1. SHA-256 hash of the request body → base64
	bodyHash := sha256.Sum256(body)
	contentHash := base64.StdEncoding.EncodeToString(bodyHash[:])

	// 2. UTC date in RFC1123 format
	utcDate := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")

	// 3. String to sign: METHOD\npath?query\ndate;host;contentHash
	host := req.URL.Host
	pathAndQuery := req.URL.RequestURI()
	stringToSign := fmt.Sprintf("%s\n%s\n%s;%s;%s",
		req.Method, pathAndQuery, utcDate, host, contentHash)

	// 4. HMAC-SHA256 over stringToSign using the decoded access key
	mac := hmac.New(sha256.New, s.accessKey)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// 5. Set required headers
	req.Header.Set("x-ms-date", utcDate)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=%s",
		signature,
	))
	return nil
}

func parseConnStr(s string) (endpoint, accessKey string, err error) {
	for _, part := range strings.Split(s, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "endpoint":
			endpoint = kv[1]
		case "accesskey":
			accessKey = kv[1]
		}
	}
	if endpoint == "" || accessKey == "" {
		return "", "", fmt.Errorf("ACS_CONNECTION_STRING missing endpoint or accesskey")
	}
	return endpoint, accessKey, nil
}
