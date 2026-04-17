package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	domainuser "github.com/sentinel-health-engine/user-service/internal/domain/user"
	appuser "github.com/sentinel-health-engine/user-service/internal/application/user"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// UserResponse is the API representation of a User.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	FullName  string    `json:"fullName"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

func toUserResponse(u *domainuser.User) UserResponse {
	return UserResponse{
		ID:        u.ID(),
		Email:     u.Email(),
		Role:      string(u.Role()),
		FirstName: u.FirstName(),
		LastName:  u.LastName(),
		FullName:  u.FullName(),
		IsActive:  u.IsActive(),
		CreatedAt: u.CreatedAt(),
	}
}

// UserHandler handles user profile endpoints.
type UserHandler struct {
	getMe *appuser.GetMeUseCase
}

func NewUserHandler(getMe *appuser.GetMeUseCase) *UserHandler {
	return &UserHandler{getMe: getMe}
}

// Register mounts all user routes onto the given router group.
func (h *UserHandler) Register(rg *gin.RouterGroup) {
	protected := rg.Group("", middleware.RequireAuth())
	protected.GET("/users/me", h.getProfile)
}

// getProfile handles GET /v1/users/me
func (h *UserHandler) getProfile(c *gin.Context) {
	u, err := h.getMe.Execute(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(u))
}
