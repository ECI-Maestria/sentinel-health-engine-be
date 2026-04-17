package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
	appdevice "github.com/sentinel-health-engine/user-service/internal/application/device"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// DeviceResponse is the API representation of a Device.
type DeviceResponse struct {
	ID               string     `json:"id"`
	DeviceIdentifier string     `json:"deviceIdentifier"`
	Platform         string     `json:"platform"`
	Name             string     `json:"name"`
	IsActive         bool       `json:"isActive"`
	LastSeenAt       *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

func toDeviceResponse(d *domaindevice.Device) DeviceResponse {
	return DeviceResponse{
		ID:               d.ID(),
		DeviceIdentifier: d.DeviceIdentifier(),
		Platform:         string(d.Platform()),
		Name:             d.Name(),
		IsActive:         d.IsActive(),
		LastSeenAt:       d.LastSeenAt(),
		CreatedAt:        d.CreatedAt(),
	}
}

// DeviceHandler handles device registration endpoints.
type DeviceHandler struct {
	register *appdevice.RegisterDeviceUseCase
	list     *appdevice.ListDevicesUseCase
}

func NewDeviceHandler(register *appdevice.RegisterDeviceUseCase, list *appdevice.ListDevicesUseCase) *DeviceHandler {
	return &DeviceHandler{register: register, list: list}
}

// Register mounts device routes. All require authentication.
func (h *DeviceHandler) Register(rg *gin.RouterGroup) {
	protected := rg.Group("", middleware.RequireAuth())
	protected.POST("/devices/register", h.registerDevice)
	protected.GET("/devices", h.listDevices)
}

// registerDevice handles POST /v1/devices/register
// Called automatically by the mobile app on every login to register/update the device.
func (h *DeviceHandler) registerDevice(c *gin.Context) {
	var body struct {
		DeviceIdentifier string `json:"deviceIdentifier" binding:"required"`
		FCMToken         string `json:"fcmToken"         binding:"required"`
		Platform         string `json:"platform"         binding:"required"`
		Name             string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d, err := h.register.Execute(c.Request.Context(), appdevice.RegisterDeviceCommand{
		UserID:           middleware.CurrentUserID(c),
		DeviceIdentifier: body.DeviceIdentifier,
		FCMToken:         body.FCMToken,
		Platform:         body.Platform,
		Name:             body.Name,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toDeviceResponse(d))
}

// listDevices handles GET /v1/devices
func (h *DeviceHandler) listDevices(c *gin.Context) {
	devices, err := h.list.Execute(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]DeviceResponse, 0, len(devices))
	for _, d := range devices {
		response = append(response, toDeviceResponse(d))
	}
	c.JSON(http.StatusOK, gin.H{"devices": response})
}
