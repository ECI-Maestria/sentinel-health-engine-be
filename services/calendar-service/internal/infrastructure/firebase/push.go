// Package firebase implements push notification delivery via Firebase Cloud Messaging.
package firebase

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	firebasesdk "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// PushNotifier sends push notifications via FCM.
type PushNotifier struct {
	client *messaging.Client
	logger *zap.Logger
}

// NewPushNotifier initializes Firebase from credentials.
// Set FIREBASE_CREDENTIALS_FILE (path) or FIREBASE_CREDENTIALS_JSON (base64).
func NewPushNotifier(ctx context.Context, logger *zap.Logger) (*PushNotifier, error) {
	var opt option.ClientOption

	switch {
	case os.Getenv("FIREBASE_CREDENTIALS_FILE") != "":
		opt = option.WithCredentialsFile(os.Getenv("FIREBASE_CREDENTIALS_FILE"))

	case os.Getenv("FIREBASE_CREDENTIALS_JSON") != "":
		decoded, err := base64.StdEncoding.DecodeString(os.Getenv("FIREBASE_CREDENTIALS_JSON"))
		if err != nil {
			return nil, fmt.Errorf("decode FIREBASE_CREDENTIALS_JSON: %w", err)
		}
		opt = option.WithCredentialsJSON(decoded)

	default:
		return nil, fmt.Errorf("firebase credentials not set: use FIREBASE_CREDENTIALS_FILE or FIREBASE_CREDENTIALS_JSON")
	}

	app, err := firebasesdk.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("init Firebase app: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("get FCM client: %w", err)
	}

	logger.Info("Firebase FCM initialized")
	return &PushNotifier{client: msgClient, logger: logger}, nil
}

// Send sends a push notification to a device identified by fcmToken.
func (n *PushNotifier) Send(ctx context.Context, fcmToken, title, body string, data map[string]string) error {
	if fcmToken == "" {
		n.logger.Warn("skipping push: FCM token is empty")
		return nil
	}

	badge := 1
	msg := &messaging.Message{
		Token: fcmToken,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:     "default",
				ChannelID: "sentinel_reminders",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
					Badge: &badge,
				},
			},
		},
	}

	msgID, err := n.client.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("FCM send failed: %w", err)
	}

	n.logger.Info("push notification sent",
		zap.String("messageId", msgID),
		zap.String("title", title),
	)
	return nil
}
