package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	appcart "github.com/sentinel-health-engine/user-service/internal/application/caretaker"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// CaretakerRelationshipResponse is the API representation of the relationship.
type CaretakerRelationshipResponse struct {
	PatientID   string    `json:"patientId"`
	CaretakerID string    `json:"caretakerId"`
	FullName    string    `json:"fullName"`
	Email       string    `json:"email"`
	LinkedAt    time.Time `json:"linkedAt"`
}

// MyPatientResponse is the API representation of a patient visible to a caretaker.
type MyPatientResponse struct {
	PatientID string    `json:"patientId"`
	FullName  string    `json:"fullName"`
	Email     string    `json:"email"`
	LinkedAt  time.Time `json:"linkedAt"`
}

// CaretakerHandler handles caretaker account creation and relationship endpoints.
type CaretakerHandler struct {
	create     *appcart.CreateCaretakerUseCase
	link       *appcart.LinkCaretakerUseCase
	unlink     *appcart.UnlinkCaretakerUseCase
	list       *appcart.ListCaretakersUseCase
	myPatients *appcart.GetMyPatientsUseCase
}

func NewCaretakerHandler(
	create *appcart.CreateCaretakerUseCase,
	link *appcart.LinkCaretakerUseCase,
	unlink *appcart.UnlinkCaretakerUseCase,
	list *appcart.ListCaretakersUseCase,
	myPatients *appcart.GetMyPatientsUseCase,
) *CaretakerHandler {
	return &CaretakerHandler{create: create, link: link, unlink: unlink, list: list, myPatients: myPatients}
}

// Register mounts caretaker routes.
func (h *CaretakerHandler) Register(rg *gin.RouterGroup) {
	// Public — no authentication required.
	rg.POST("/caretakers/register", h.selfRegister)

	protected := rg.Group("", middleware.RequireAuth())
	protected.POST("/caretakers", middleware.RequireRole("DOCTOR"), h.createCaretaker)
	protected.POST("/patients/:id/caretakers", middleware.RequireRole("DOCTOR", "PATIENT"), h.linkCaretaker)
	protected.DELETE("/patients/:id/caretakers/:caretakerId", middleware.RequireRole("DOCTOR", "PATIENT"), h.unlinkCaretaker)
	protected.GET("/patients/:id/caretakers", h.listCaretakers)
	protected.GET("/caretakers/me/patients", middleware.RequireRole("CARETAKER"), h.getMyPatients)
}

// selfRegister handles POST /v1/caretakers/register (public)
// Allows anyone to create their own CARETAKER account.
// The account is inactive from a functional perspective until linked to a patient.
func (h *CaretakerHandler) selfRegister(c *gin.Context) {
	var body struct {
		FirstName string `json:"firstName" binding:"required"`
		LastName  string `json:"lastName"  binding:"required"`
		Email     string `json:"email"     binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.create.Execute(c.Request.Context(), appcart.CreateCaretakerCommand{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(u))
}

// createCaretaker handles POST /v1/caretakers (DOCTOR only)
func (h *CaretakerHandler) createCaretaker(c *gin.Context) {
	var body struct {
		FirstName string `json:"firstName" binding:"required"`
		LastName  string `json:"lastName"  binding:"required"`
		Email     string `json:"email"     binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.create.Execute(c.Request.Context(), appcart.CreateCaretakerCommand{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(u))
}

// linkCaretaker handles POST /v1/patients/:id/caretakers
// Accepts either caretakerId (UUID) or caretakerEmail to identify the caretaker.
func (h *CaretakerHandler) linkCaretaker(c *gin.Context) {
	var body struct {
		CaretakerID    string `json:"caretakerId"`
		CaretakerEmail string `json:"caretakerEmail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.CaretakerID == "" && body.CaretakerEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caretakerId or caretakerEmail is required"})
		return
	}

	err := h.link.Execute(c.Request.Context(), appcart.LinkCaretakerCommand{
		PatientID:      c.Param("id"),
		CaretakerID:    body.CaretakerID,
		CaretakerEmail: body.CaretakerEmail,
		LinkedBy:       middleware.CurrentUserID(c),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "caretaker linked successfully"})
}

// unlinkCaretaker handles DELETE /v1/patients/:id/caretakers/:caretakerId
func (h *CaretakerHandler) unlinkCaretaker(c *gin.Context) {
	err := h.unlink.Execute(c.Request.Context(), appcart.UnlinkCaretakerCommand{
		PatientID:   c.Param("id"),
		CaretakerID: c.Param("caretakerId"),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "caretaker unlinked successfully"})
}

// listCaretakers handles GET /v1/patients/:id/caretakers
func (h *CaretakerHandler) listCaretakers(c *gin.Context) {
	caretakers, err := h.list.Execute(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]CaretakerRelationshipResponse, 0, len(caretakers))
	for _, cw := range caretakers {
		response = append(response, CaretakerRelationshipResponse{
			PatientID:   cw.Relationship.PatientID(),
			CaretakerID: cw.Relationship.CaretakerID(),
			FullName:    cw.User.FullName(),
			Email:       cw.User.Email(),
			LinkedAt:    cw.Relationship.CreatedAt(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"caretakers": response})
}

// getMyPatients handles GET /v1/caretakers/me/patients (CARETAKER only)
// Returns all patients linked to the authenticated caretaker.
// An empty array means the caretaker has not been linked to any patient yet.
func (h *CaretakerHandler) getMyPatients(c *gin.Context) {
	summaries, err := h.myPatients.Execute(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]MyPatientResponse, 0, len(summaries))
	for _, s := range summaries {
		response = append(response, MyPatientResponse{
			PatientID: s.User.ID(),
			FullName:  s.User.FullName(),
			Email:     s.User.Email(),
			LinkedAt:  s.Relationship.CreatedAt(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"patients": response,
		"isLinked": len(response) > 0,
	})
}
