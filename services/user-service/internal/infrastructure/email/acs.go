// Package email implements welcome email delivery via Azure Communication Services.
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

// ACSSender sends emails via Azure Communication Services using HMAC-SHA256 auth.
type ACSSender struct {
	endpoint      string
	accessKey     []byte
	senderAddress string
	client        *http.Client
	logger        *zap.Logger
}

// NewACSSender initialises the sender from environment variables.
// Required env vars: ACS_CONNECTION_STRING, ACS_SENDER_ADDRESS
func NewACSSender(logger *zap.Logger) (*ACSSender, error) {
	connStr := os.Getenv("ACS_CONNECTION_STRING")
	if connStr == "" {
		return nil, fmt.Errorf("ACS_CONNECTION_STRING is required")
	}
	senderAddr := os.Getenv("ACS_SENDER_ADDRESS")
	if senderAddr == "" {
		return nil, fmt.Errorf("ACS_SENDER_ADDRESS is required")
	}

	endpoint, keyB64, err := parseConnectionString(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse ACS connection string: %w", err)
	}
	accessKey, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode ACS access key: %w", err)
	}

	return &ACSSender{
		endpoint:      endpoint,
		accessKey:     accessKey,
		senderAddress: senderAddr,
		client:        &http.Client{Timeout: 15 * time.Second},
		logger:        logger,
	}, nil
}

// SendWelcome sends a welcome email with the temporary password.
func (s *ACSSender) SendWelcome(ctx context.Context, toEmail, fullName, temporaryPassword string) error {
	subject := "Bienvenido a Sentinel Health — Sus credenciales de acceso"
	html := buildWelcomeEmailHTML(fullName, toEmail, temporaryPassword)
	return s.send(ctx, toEmail, subject, html)
}

// SendPasswordResetCode sends a 6-digit OTP code to the user for password reset.
func (s *ACSSender) SendPasswordResetCode(ctx context.Context, toEmail, fullName, code string) error {
	subject := "Sentinel Health — Código de restablecimiento de contraseña"
	html := buildResetEmailHTML(fullName, code)
	return s.send(ctx, toEmail, subject, html)
}

func (s *ACSSender) send(ctx context.Context, toEmail, subject, htmlBody string) error {
	payload := map[string]any{
		"senderAddress": s.senderAddress,
		"content": map[string]string{
			"subject":   subject,
			"html":      htmlBody,
		},
		"recipients": map[string]any{
			"to": []map[string]string{{"address": toEmail}},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := s.endpoint + "/emails:send?api-version=2023-03-31"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := s.signRequest(req, body); err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ACS returned %d: %s", resp.StatusCode, string(raw))
	}

	s.logger.Info("welcome email sent via ACS", zap.String("to", toEmail))
	return nil
}

func (s *ACSSender) signRequest(req *http.Request, body []byte) error {
	bodyHash := sha256.Sum256(body)
	contentHash := base64.StdEncoding.EncodeToString(bodyHash[:])
	utcDate := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	host := req.URL.Host
	pathAndQuery := req.URL.RequestURI()

	stringToSign := fmt.Sprintf("%s\n%s\n%s;%s;%s",
		req.Method, pathAndQuery, utcDate, host, contentHash)

	s.logger.Info("ACS HMAC debug",
		zap.String("endpoint", s.endpoint),
		zap.String("fullURL", req.URL.String()),
		zap.String("host", host),
		zap.String("pathAndQuery", pathAndQuery),
		zap.String("utcDate", utcDate),
		zap.String("contentHash", contentHash),
		zap.String("stringToSign", stringToSign),
		zap.Int("accessKeyLen", len(s.accessKey)),
	)

	mac := hmac.New(sha256.New, s.accessKey)
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("x-ms-date", utcDate)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 SignedHeaders=x-ms-date;host;x-ms-content-sha256&Signature=%s", signature))
	return nil
}

func parseConnectionString(connStr string) (endpoint, accessKey string, err error) {
	for _, part := range strings.Split(connStr, ";") {
		if strings.HasPrefix(part, "endpoint=") {
			endpoint = strings.TrimRight(strings.TrimPrefix(part, "endpoint="), "/")
		} else if strings.HasPrefix(part, "accesskey=") {
			accessKey = strings.TrimPrefix(part, "accesskey=")
		}
	}
	if endpoint == "" || accessKey == "" {
		return "", "", fmt.Errorf("missing endpoint or accesskey in connection string")
	}
	return endpoint, accessKey, nil
}

func buildResetEmailHTML(fullName, code string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;">
  <div style="background:#0ea5e9;color:white;padding:24px;border-radius:8px 8px 0 0;">
    <h1 style="margin:0;font-size:22px;">Restablecer Contraseña</h1>
    <p style="margin:4px 0 0;opacity:0.85;">Sentinel Health Engine</p>
  </div>
  <div style="background:#f9f9f9;padding:24px;border-radius:0 0 8px 8px;">
    <p>Hola <strong>%s</strong>,</p>
    <p>Recibimos una solicitud para restablecer la contraseña de su cuenta. Ingrese el siguiente código en la aplicación:</p>
    <div style="background:#fff;border:2px solid #0ea5e9;border-radius:8px;padding:20px;text-align:center;margin:24px 0;">
      <p style="margin:0 0 6px;font-size:13px;color:#6b7280;">Su código de verificación</p>
      <p style="margin:0;font-size:36px;font-weight:bold;letter-spacing:10px;font-family:monospace;color:#111827;">%s</p>
    </div>
    <p style="color:#6b7280;font-size:13px;">Este código expira en <strong>1 minuto</strong>. Si no solicitó este cambio, puede ignorar este correo.</p>
    <hr style="border:none;border-top:1px solid #e5e7eb;margin:20px 0;"/>
    <p style="color:#888;font-size:11px;">Este es un mensaje automático. Por favor no responda a este correo.</p>
  </div>
</body></html>`, fullName, code)
}

func buildWelcomeEmailHTML(fullName, email, password string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;">
  <div style="background:#0ea5e9;color:white;padding:24px;border-radius:8px 8px 0 0;">
    <h1 style="margin:0;font-size:22px;">Bienvenido a Sentinel Health</h1>
    <p style="margin:4px 0 0;opacity:0.85;">Sistema de Monitoreo de Signos Vitales</p>
  </div>
  <div style="background:#f9f9f9;padding:24px;border-radius:0 0 8px 8px;">
    <p>Hola <strong>%s</strong>,</p>
    <p>Su cuenta ha sido creada exitosamente. A continuación encontrará sus credenciales de acceso:</p>
    <div style="background:#fff;border:1px solid #e5e7eb;border-radius:6px;padding:16px;margin:16px 0;">
      <p style="margin:4px 0;"><strong>Correo:</strong> %s</p>
      <p style="margin:4px 0;"><strong>Contraseña temporal:</strong> <code style="background:#f3f4f6;padding:2px 6px;border-radius:4px;">%s</code></p>
    </div>
    <p>Puede cambiar su contraseña en cualquier momento desde la aplicación en <em>Perfil → Cambiar contraseña</em>.</p>
    <hr style="border:none;border-top:1px solid #e5e7eb;margin:20px 0;"/>
    <p style="color:#888;font-size:11px;">Este es un mensaje automático. Por favor no responda a este correo.</p>
  </div>
</body></html>`, fullName, email, password)
}
