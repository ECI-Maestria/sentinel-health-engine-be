package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appcart "github.com/sentinel-health-engine/user-service/internal/application/caretaker"
	appdevice "github.com/sentinel-health-engine/user-service/internal/application/device"
	"github.com/sentinel-health-engine/user-service/internal/application/patient"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// PatientHandler handles patient management endpoints (Doctor-only).
type PatientHandler struct {
	create         *patient.CreatePatientUseCase
	list           *patient.ListPatientsUseCase
	get            *patient.GetPatientUseCase
	listDevices    *appdevice.ListDevicesUseCase
	listCaretakers *appcart.ListCaretakersUseCase
}

func NewPatientHandler(
	create *patient.CreatePatientUseCase,
	list *patient.ListPatientsUseCase,
	get *patient.GetPatientUseCase,
	listDevices *appdevice.ListDevicesUseCase,
	listCaretakers *appcart.ListCaretakersUseCase,
) *PatientHandler {
	return &PatientHandler{
		create:         create,
		list:           list,
		get:            get,
		listDevices:    listDevices,
		listCaretakers: listCaretakers,
	}
}

// Register mounts patient routes. All require DOCTOR role.
func (h *PatientHandler) Register(rg *gin.RouterGroup) {
	doctorRoutes := rg.Group("", middleware.RequireAuth(), middleware.RequireRole("DOCTOR"))
	doctorRoutes.POST("/patients", h.createPatient)
	doctorRoutes.GET("/patients", h.listPatients)
	doctorRoutes.GET("/patients/:id", h.getPatient)
	doctorRoutes.GET("/patients/:id/profile/complete", h.getCompleteProfile)
}

// createPatient handles POST /v1/patients
func (h *PatientHandler) createPatient(c *gin.Context) {
	var body struct {
		FirstName string `json:"firstName" binding:"required"`
		LastName  string `json:"lastName"  binding:"required"`
		Email     string `json:"email"     binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.create.Execute(c.Request.Context(), patient.CreatePatientCommand{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(result.Patient))
}

// listPatients handles GET /v1/patients
func (h *PatientHandler) listPatients(c *gin.Context) {
	patients, err := h.list.Execute(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]UserResponse, 0, len(patients))
	for _, p := range patients {
		response = append(response, toUserResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"patients": response})
}

// getPatient handles GET /v1/patients/:id
func (h *PatientHandler) getPatient(c *gin.Context) {
	p, err := h.get.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(p))
}

// getCompleteProfile handles GET /v1/patients/:id/profile/complete
func (h *PatientHandler) getCompleteProfile(c *gin.Context) {
	patientID := c.Param("id")
	ctx := c.Request.Context()

	p, err := h.get.Execute(ctx, patientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	devices, err := h.listDevices.Execute(ctx, patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	caretakers, err := h.listCaretakers.Execute(ctx, patientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deviceResponses := make([]DeviceResponse, 0, len(devices))
	for _, d := range devices {
		deviceResponses = append(deviceResponses, toDeviceResponse(d))
	}

	caretakerResponses := make([]CaretakerRelationshipResponse, 0, len(caretakers))
	for _, cw := range caretakers {
		caretakerResponses = append(caretakerResponses, CaretakerRelationshipResponse{
			PatientID:   cw.Relationship.PatientID(),
			CaretakerID: cw.Relationship.CaretakerID(),
			FullName:    cw.User.FullName(),
			Email:       cw.User.Email(),
			LinkedAt:    cw.Relationship.CreatedAt(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"patient":    toUserResponse(p),
		"devices":    deviceResponses,
		"caretakers": caretakerResponses,
	})
}
