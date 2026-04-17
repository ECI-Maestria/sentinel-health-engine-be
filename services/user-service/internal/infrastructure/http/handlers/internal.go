package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appinternal "github.com/sentinel-health-engine/user-service/internal/application/svcinternal"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// InternalHandler serves endpoints exclusively for service-to-service communication.
// Protected by API key — not accessible by end users.
type InternalHandler struct {
	deviceValidator *appinternal.ValidateDeviceUseCase
	contactsGetter  *appinternal.GetPatientContactsUseCase
}

func NewInternalHandler(validateDevice *appinternal.ValidateDeviceUseCase, getContacts *appinternal.GetPatientContactsUseCase) *InternalHandler {
	return &InternalHandler{deviceValidator: validateDevice, contactsGetter: getContacts}
}

// Register mounts internal routes. All require the internal API key.
func (h *InternalHandler) Register(rg *gin.RouterGroup) {
	internal := rg.Group("/internal", middleware.RequireInternalAPIKey())
	internal.GET("/devices/:identifier", h.validateDevice)
	internal.GET("/patients/:id/contacts", h.getPatientContacts)
}

// validateDevice handles GET /v1/internal/devices/:identifier
// Used by telemetry-service to resolve IoT device identifier → patient ID.
func (h *InternalHandler) validateDevice(c *gin.Context) {
	result, err := h.deviceValidator.Execute(c.Request.Context(), c.Param("identifier"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"patientId": result.PatientID,
		"userId":    result.UserID,
		"deviceId":  result.DeviceID,
		"isActive":  result.IsActive,
	})
}

// getPatientContacts handles GET /v1/internal/patients/:id/contacts
// Used by alerts-service to get push/email recipients for a patient.
func (h *InternalHandler) getPatientContacts(c *gin.Context) {
	contacts, err := h.contactsGetter.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	type contactDTO struct {
		Email    string `json:"email"`
		FCMToken string `json:"fcmToken,omitempty"`
	}

	dtos := make([]contactDTO, 0, len(contacts))
	for _, ct := range contacts {
		dtos = append(dtos, contactDTO{Email: ct.Email, FCMToken: ct.FCMToken})
	}

	c.JSON(http.StatusOK, gin.H{"contacts": dtos})
}
