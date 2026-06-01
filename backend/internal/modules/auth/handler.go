package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

func (h *Handler) RegisterPublic(group *gin.RouterGroup) {
	group.POST("/auth/login", h.login)
}

func (h *Handler) RegisterPrivate(group *gin.RouterGroup) {
	group.GET("/me", h.me)
	group.POST("/auth/refresh", h.refresh)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式不正确"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)

	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名"})
		return
	}
	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入密码"})
		return
	}
	var user models.User
	err := h.store.DB.Where("username = ?", req.Username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || !verifyPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.EnsureDefaultDataForUser(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	respondWithToken(c, user)
}

func (h *Handler) me(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var user models.User
	if err := h.store.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) refresh(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var user models.User
	if err := h.store.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	respondWithToken(c, user)
}

func (h *Handler) EnsureBootstrapAdmin() error {
	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	name := strings.TrimSpace(os.Getenv("ADMIN_NAME"))
	if username == "" {
		return errors.New("ADMIN_USERNAME is required")
	}
	if password == "" {
		return errors.New("ADMIN_PASSWORD is required")
	}
	if name == "" {
		name = "好好用户"
	}

	var user models.User
	err := h.store.DB.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash, err := hashPassword(password)
		if err != nil {
			return err
		}
		user = models.User{Username: username, PasswordHash: hash, Name: name}
		if err := h.store.DB.Create(&user).Error; err != nil {
			return err
		}
		return h.store.EnsureDefaultDataForUser(user.ID)
	}
	if err != nil {
		return err
	}
	if user.PasswordHash == "" {
		hash, err := hashPassword(password)
		if err != nil {
			return err
		}
		user.PasswordHash = hash
		if strings.TrimSpace(user.Name) == "" {
			user.Name = name
		}
		if err := h.store.DB.Save(&user).Error; err != nil {
			return err
		}
	}
	return h.store.EnsureDefaultDataForUser(user.ID)
}

func respondWithToken(c *gin.Context, user models.User) {
	token, err := middleware.BuildToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}
