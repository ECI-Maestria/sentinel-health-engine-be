package domain

import "context"

// AlertRepository is the port for persisting alerts.
type AlertRepository interface {
	Save(ctx context.Context, alert *Alert) error
	Update(ctx context.Context, alert *Alert) error
}
