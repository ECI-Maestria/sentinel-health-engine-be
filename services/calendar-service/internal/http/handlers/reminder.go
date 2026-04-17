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

// ReminderHandler handles HTTP requests for reminders.
type ReminderHandler struct {
	repo   *postgres.ReminderRepository
	logger *zap.Logger
}

// NewReminderHandler creates a new ReminderHandler.
func NewReminderHandler(repo *postgres.ReminderRepository, logger *zap.Logger) *ReminderHandler {
	return &ReminderHandler{repo: repo, logger: logger}
}

// reminderResponse is the JSON shape returned for a single reminder.
type reminderResponse struct {
	ID         string  `json:"id"`
	PatientID  string  `json:"patientId"`
	CreatedBy  string  `json:"createdBy"`
	Title      string  `json:"title"`
	Message    string  `json:"message"`
	ReminderAt string  `json:"reminderAt"`
	Recurrence string  `json:"recurrence"`
	Status     string  `json:"status"`
	SentAt     *string `json:"sentAt"`
	CreatedAt  string  `json:"createdAt"`
}

func toReminderResponse(r *domain.Reminder) reminderResponse {
	resp := reminderResponse{
		ID:         r.ID,
		PatientID:  r.PatientID,
		CreatedBy:  r.CreatedBy,
		Title:      r.Title,
		Message:    r.Message,
		ReminderAt: r.ReminderAt.UTC().Format(time.RFC3339),
		Recurrence: string(r.Recurrence),
		Status:     string(r.Status),
		CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
	}
	if r.SentAt != nil {
		s := r.SentAt.UTC().Format(time.RFC3339)
		resp.SentAt = &s
	}
	return resp
}

// Create handles POST /v1/patients/:id/reminders
func (h *ReminderHandler) Create(c *gin.Context) {
	patientID := c.Param("id")
	createdBy := middleware.CurrentUserID(c)

	var req struct {
		Title      string `json:"title"      binding:"required"`
		Message    string `json:"message"    binding:"required"`
		ReminderAt string `json:"reminderAt" binding:"required"`
		Recurrence string `json:"recurrence" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reminderAt, err := time.Parse(time.RFC3339, req.ReminderAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reminderAt must be RFC3339"})
		return
	}

	rem, err := domain.NewReminder(patientID, createdBy, req.Title, req.Message,
		reminderAt, domain.Recurrence(req.Recurrence))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Save(c.Request.Context(), rem); err != nil {
		h.logger.Error("create reminder", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reminder"})
		return
	}

	c.JSON(http.StatusCreated, toReminderResponse(rem))
}

// List handles GET /v1/patients/:id/reminders
// Optional query params:
//
//	?period=day|week|month|year  — filter by time period
//	?date=YYYY-MM-DD             — reference date for the period (default: today)
func (h *ReminderHandler) List(c *gin.Context) {
	patientID := c.Param("id")

	from, to, err := periodBounds(c.Query("period"), c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reminders, err := h.repo.ListByPatient(c.Request.Context(), patientID)
	if err != nil {
		h.logger.Error("list reminders", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reminders"})
		return
	}

	resp := make([]reminderResponse, 0, len(reminders))
	for _, r := range reminders {
		if inRange(r.ReminderAt.UTC(), from, to) {
			resp = append(resp, toReminderResponse(r))
		}
	}

	c.JSON(http.StatusOK, gin.H{"reminders": resp})
}

// ListToday handles GET /v1/patients/:id/reminders/today
// Kept for backwards compatibility — equivalent to ?period=day with today's date.
func (h *ReminderHandler) ListToday(c *gin.Context) {
	patientID := c.Param("id")

	from, to, _ := periodBounds("day", "")

	all, err := h.repo.ListByPatient(c.Request.Context(), patientID)
	if err != nil {
		h.logger.Error("list today reminders", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reminders"})
		return
	}

	resp := make([]reminderResponse, 0)
	for _, r := range all {
		if inRange(r.ReminderAt.UTC(), from, to) {
			resp = append(resp, toReminderResponse(r))
		}
	}

	c.JSON(http.StatusOK, gin.H{"reminders": resp})
}

// GetByID handles GET /v1/patients/:id/reminders/:remId
func (h *ReminderHandler) GetByID(c *gin.Context) {
	remID := c.Param("remId")

	rem, err := h.repo.FindByID(c.Request.Context(), remID)
	if err != nil {
		h.logger.Error("get reminder by id", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve reminder"})
		return
	}
	if rem == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}

	c.JSON(http.StatusOK, toReminderResponse(rem))
}

// Update handles PUT /v1/patients/:id/reminders/:remId
func (h *ReminderHandler) Update(c *gin.Context) {
	remID := c.Param("remId")

	rem, err := h.repo.FindByID(c.Request.Context(), remID)
	if err != nil {
		h.logger.Error("find reminder for update", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve reminder"})
		return
	}
	if rem == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}
	if rem.Status != domain.ReminderPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only PENDING reminders can be updated"})
		return
	}

	var req struct {
		Title      *string `json:"title"`
		Message    *string `json:"message"`
		ReminderAt *string `json:"reminderAt"`
		Recurrence *string `json:"recurrence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != nil {
		rem.Title = *req.Title
	}
	if req.Message != nil {
		rem.Message = *req.Message
	}
	if req.ReminderAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ReminderAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "reminderAt must be RFC3339"})
			return
		}
		rem.ReminderAt = t.UTC()
	}
	if req.Recurrence != nil {
		rem.Recurrence = domain.Recurrence(*req.Recurrence)
	}

	if err := h.repo.Update(c.Request.Context(), rem); err != nil {
		h.logger.Error("update reminder", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update reminder"})
		return
	}

	c.JSON(http.StatusOK, toReminderResponse(rem))
}

// Cancel handles DELETE /v1/patients/:id/reminders/:remId — marks reminder as CANCELLED.
func (h *ReminderHandler) Cancel(c *gin.Context) {
	remID := c.Param("remId")

	rem, err := h.repo.FindByID(c.Request.Context(), remID)
	if err != nil {
		h.logger.Error("find reminder for cancel", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve reminder"})
		return
	}
	if rem == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}

	rem.Status = domain.ReminderCancelled
	if err := h.repo.Update(c.Request.Context(), rem); err != nil {
		h.logger.Error("cancel reminder", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel reminder"})
		return
	}

	c.Status(http.StatusNoContent)
}
