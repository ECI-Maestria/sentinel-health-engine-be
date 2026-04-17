package device

import "context"

// Repository is the port for Device persistence.
type Repository interface {
	Save(ctx context.Context, d *Device) error
	Update(ctx context.Context, d *Device) error
	FindByID(ctx context.Context, id string) (*Device, error)
	FindByUserIDAndIdentifier(ctx context.Context, userID, deviceIdentifier string) (*Device, error)
	FindByIdentifier(ctx context.Context, deviceIdentifier string) (*Device, error)
	ListByUserID(ctx context.Context, userID string) ([]*Device, error)
	// ListActiveByUserID returns only active devices with FCM tokens for a user.
	ListActiveByUserID(ctx context.Context, userID string) ([]*Device, error)
}
