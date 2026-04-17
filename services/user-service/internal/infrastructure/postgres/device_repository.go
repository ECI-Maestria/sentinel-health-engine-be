package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
)

// DeviceRepository implements device.Repository backed by PostgreSQL.
type DeviceRepository struct {
	pool *pgxpool.Pool
}

func NewDeviceRepository(pool *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{pool: pool}
}

func (r *DeviceRepository) Save(ctx context.Context, d *domaindevice.Device) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO devices (id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, d.ID(), d.UserID(), d.DeviceIdentifier(), d.FCMToken(), string(d.Platform()),
		d.Name(), d.IsActive(), d.LastSeenAt(), d.CreatedAt(), d.UpdatedAt())
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	return nil
}

func (r *DeviceRepository) Update(ctx context.Context, d *domaindevice.Device) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE devices SET
			fcm_token    = $2,
			name         = $3,
			is_active    = $4,
			last_seen_at = $5,
			updated_at   = $6
		WHERE id = $1
	`, d.ID(), d.FCMToken(), d.Name(), d.IsActive(), d.LastSeenAt(), d.UpdatedAt())
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("device %s not found", d.ID())
	}
	return nil
}

func (r *DeviceRepository) FindByID(ctx context.Context, id string) (*domaindevice.Device, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at
		FROM devices WHERE id = $1
	`, id)
	return scanDevice(row)
}

func (r *DeviceRepository) FindByUserIDAndIdentifier(ctx context.Context, userID, deviceIdentifier string) (*domaindevice.Device, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at
		FROM devices WHERE user_id = $1 AND device_identifier = $2
	`, userID, deviceIdentifier)
	return scanDevice(row)
}

func (r *DeviceRepository) FindByIdentifier(ctx context.Context, deviceIdentifier string) (*domaindevice.Device, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at
		FROM devices WHERE device_identifier = $1 AND is_active = true
		ORDER BY updated_at DESC
		LIMIT 1
	`, deviceIdentifier)
	return scanDevice(row)
}

func (r *DeviceRepository) ListByUserID(ctx context.Context, userID string) ([]*domaindevice.Device, error) {
	return r.listDevices(ctx, `
		SELECT id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at
		FROM devices WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
}

func (r *DeviceRepository) ListActiveByUserID(ctx context.Context, userID string) ([]*domaindevice.Device, error) {
	return r.listDevices(ctx, `
		SELECT id, user_id, device_identifier, fcm_token, platform, name, is_active, last_seen_at, created_at, updated_at
		FROM devices WHERE user_id = $1 AND is_active = true AND fcm_token IS NOT NULL AND fcm_token <> ''
		ORDER BY COALESCE(last_seen_at, created_at) DESC
	`, userID)
}

func (r *DeviceRepository) listDevices(ctx context.Context, query string, args ...any) ([]*domaindevice.Device, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var devices []*domaindevice.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func scanDevice(row interface {
	Scan(dest ...any) error
}) (*domaindevice.Device, error) {
	var (
		id               string
		userID           string
		deviceIdentifier string
		fcmToken         *string
		platform         string
		name             *string
		isActive         bool
		lastSeenAt       *time.Time
		createdAt        time.Time
		updatedAt        time.Time
	)

	if err := row.Scan(&id, &userID, &deviceIdentifier, &fcmToken, &platform, &name, &isActive, &lastSeenAt, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("scan device: %w", err)
	}

	fcm := ""
	if fcmToken != nil {
		fcm = *fcmToken
	}
	n := ""
	if name != nil {
		n = *name
	}

	return domaindevice.Reconstitute(id, userID, deviceIdentifier, fcm, n, domaindevice.Platform(platform), isActive, lastSeenAt, createdAt, updatedAt), nil
}
