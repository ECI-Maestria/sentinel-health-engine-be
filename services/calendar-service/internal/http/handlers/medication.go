package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sentinel-health-engine/calendar-service/internal/domain"
	"github.com/sentinel-health-engine/calendar-service/internal/http/middleware"
	"github.com/sentinel-health-engine/calendar-service/internal/postgres"
)

// MedicationHandler handles HTTP requests for medications.
type MedicationHandler struct {
	repo   *postgres.MedicationRepository
	logger *zap.Logger
}

// NewMedicationHandler creates a new MedicationHandler.
func NewMedicationHandler(repo *postgres.MedicationRepository, logger *zap.Logger) *MedicationHandler {
	return &MedicationHandler{repo: repo, logger: logger}
}

// medicationResponse is the JSON shape returned for a single medication.
type medicationResponse struct {
	ID             string   `json:"id"`
	PatientID      string   `json:"patientId"`
	PrescribedBy   string   `json:"prescribedBy"`
	Name           string   `json:"name"`
	Dosage         string   `json:"dosage"`
	Frequency      string   `json:"frequency"`
	ScheduledTimes []string `json:"scheduledTimes"`
	StartDate      string   `json:"startDate"`
	EndDate        *string  `json:"endDate"`
	Notes          string   `json:"notes"`
	IsActive       bool     `json:"isActive"`
	CreatedAt      string   `json:"createdAt"`
}

func toMedicationResponse(m *domain.Medication) medicationResponse {
	resp := medicationResponse{
		ID:             m.ID,
		PatientID:      m.PatientID,
		PrescribedBy:   m.PrescribedBy,
		Name:           m.Name,
		Dosage:         m.Dosage,
		Frequency:      string(m.Frequency),
		ScheduledTimes: m.ScheduledTimes,
		StartDate:      m.StartDate.UTC().Format("2006-01-02"),
		Notes:          m.Notes,
		IsActive:       m.IsActive,
		CreatedAt:      m.CreatedAt.UTC().Format(time.RFC3339),
	}
	if m.EndDate != nil {
		s := m.EndDate.UTC().Format("2006-01-02")
		resp.EndDate = &s
	}
	if resp.ScheduledTimes == nil {
		resp.ScheduledTimes = []string{}
	}
	return resp
}

// Create handles POST /v1/patients/:id/medications
func (h *MedicationHandler) Create(c *gin.Context) {
	patientID := c.Param("id")
	doctorID := middleware.CurrentUserID(c)

	var req struct {
		Name           string   `json:"name"           binding:"required"`
		Dosage         string   `json:"dosage"         binding:"required"`
		Frequency      string   `json:"frequency"      binding:"required"`
		ScheduledTimes []string `json:"scheduledTimes"`
		StartDate      string   `json:"startDate"      binding:"required"`
		EndDate        *string  `json:"endDate"`
		Notes          string   `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate must be in YYYY-MM-DD format"})
		return
	}

	med, err := domain.NewMedication(patientID, doctorID, req.Name, req.Dosage,
		domain.Frequency(req.Frequency), req.ScheduledTimes, startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	med.Notes = req.Notes

	if req.EndDate != nil {
		ed, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate must be in YYYY-MM-DD format"})
			return
		}
		med.EndDate = &ed
	}

	if err := h.repo.Save(c.Request.Context(), med); err != nil {
		h.logger.Error("create medication", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create medication"})
		return
	}

	c.JSON(http.StatusCreated, toMedicationResponse(med))
}

// List handles GET /v1/patients/:id/medications
func (h *MedicationHandler) List(c *gin.Context) {
	patientID := c.Param("id")
	activeOnly := c.Query("active") == "true"

	medications, err := h.repo.ListByPatient(c.Request.Context(), patientID, activeOnly)
	if err != nil {
		h.logger.Error("list medications", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list medications"})
		return
	}

	resp := make([]medicationResponse, 0, len(medications))
	for _, m := range medications {
		resp = append(resp, toMedicationResponse(m))
	}

	c.JSON(http.StatusOK, gin.H{"medications": resp})
}

// GetByID handles GET /v1/patients/:id/medications/:medId
func (h *MedicationHandler) GetByID(c *gin.Context) {
	medID := c.Param("medId")

	med, err := h.repo.FindByID(c.Request.Context(), medID)
	if err != nil {
		h.logger.Error("get medication by id", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve medication"})
		return
	}
	if med == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	c.JSON(http.StatusOK, toMedicationResponse(med))
}

// Update handles PUT /v1/patients/:id/medications/:medId
func (h *MedicationHandler) Update(c *gin.Context) {
	medID := c.Param("medId")

	med, err := h.repo.FindByID(c.Request.Context(), medID)
	if err != nil {
		h.logger.Error("find medication for update", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve medication"})
		return
	}
	if med == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	var req struct {
		Name           *string   `json:"name"`
		Dosage         *string   `json:"dosage"`
		Frequency      *string   `json:"frequency"`
		ScheduledTimes *[]string `json:"scheduledTimes"`
		StartDate      *string   `json:"startDate"`
		EndDate        *string   `json:"endDate"`
		Notes          *string   `json:"notes"`
		IsActive       *bool     `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil {
		med.Name = *req.Name
	}
	if req.Dosage != nil {
		med.Dosage = *req.Dosage
	}
	if req.Frequency != nil {
		med.Frequency = domain.Frequency(*req.Frequency)
	}
	if req.ScheduledTimes != nil {
		med.ScheduledTimes = *req.ScheduledTimes
	}
	if req.StartDate != nil {
		sd, err := time.Parse("2006-01-02", *req.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate must be in YYYY-MM-DD format"})
			return
		}
		med.StartDate = sd
	}
	if req.EndDate != nil {
		ed, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate must be in YYYY-MM-DD format"})
			return
		}
		med.EndDate = &ed
	}
	if req.Notes != nil {
		med.Notes = *req.Notes
	}
	if req.IsActive != nil {
		med.IsActive = *req.IsActive
	}

	if err := h.repo.Update(c.Request.Context(), med); err != nil {
		h.logger.Error("update medication", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update medication"})
		return
	}

	c.JSON(http.StatusOK, toMedicationResponse(med))
}

// Deactivate handles DELETE /v1/patients/:id/medications/:medId (soft delete)
func (h *MedicationHandler) Deactivate(c *gin.Context) {
	medID := c.Param("medId")

	med, err := h.repo.FindByID(c.Request.Context(), medID)
	if err != nil {
		h.logger.Error("find medication for deactivation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve medication"})
		return
	}
	if med == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	if err := h.repo.Deactivate(c.Request.Context(), medID); err != nil {
		h.logger.Error("deactivate medication", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deactivate medication"})
		return
	}

	c.Status(http.StatusNoContent)
}
