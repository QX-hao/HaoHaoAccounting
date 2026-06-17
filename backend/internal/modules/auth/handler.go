package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	store        *store.Store
	admin        config.AdminConfig
	loginLimiter *loginLimiter
	tokenService *middleware.TokenService
}

func NewHandler(s *store.Store) *Handler {
	cfg := config.Load()
	tokenService, _ := middleware.NewTokenServiceWithTTL(cfg.JWT.Secret, cfg.JWT.TTL, cfg.JWT.ClockSkew, cfg.JWT.Issuer, cfg.JWT.Audience)
	return NewHandlerWithConfig(s, cfg.Admin, cfg.LoginRateLimit, tokenService)
}

func NewHandlerWithConfig(s *store.Store, admin config.AdminConfig, limiter config.LoginRateLimitConfig, tokenService *middleware.TokenService) *Handler {
	return &Handler{store: s, admin: admin, loginLimiter: newLoginLimiterFromConfig(limiter), tokenService: tokenService}
}

func (h *Handler) RegisterPublic(group *gin.RouterGroup) {
	group.POST("/auth/login", h.login)
}

func (h *Handler) RegisterPrivate(group *gin.RouterGroup) {
	group.GET("/me", h.me)
	group.POST("/auth/refresh", h.refresh)
	group.POST("/auth/logout", h.logout)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := httputil.BindJSONBody(c, &req); err != nil {
		if middleware.HandleBodyReadError(c, err) {
			return
		}
		httputil.InvalidRequest(c, "请求体格式不正确")
		return
	}

	req.Username = strings.TrimSpace(req.Username)

	if req.Username == "" {
		httputil.BadRequest(c, "请输入用户名")
		return
	}
	if req.Password == "" {
		httputil.BadRequest(c, "请输入密码")
		return
	}

	limiterKey := loginLimiterKey(c.ClientIP(), req.Username)
	if h.loginLimiter != nil && !h.loginLimiter.Allow(limiterKey) {
		httputil.RateLimited(c, "登录失败次数过多，请稍后再试", h.loginLimiter.RetryAfter(limiterKey))
		return
	}

	var user models.User
	err := h.store.DB.Where("username = ?", req.Username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		h.recordLoginFailure(limiterKey)
		httputil.Unauthorized(c, "用户名或密码错误")
		return
	} else if err != nil {
		httputil.InternalError(c, err)
		return
	}
	if !verifyPassword(user.PasswordHash, req.Password) {
		h.recordLoginFailure(limiterKey)
		httputil.Unauthorized(c, "用户名或密码错误")
		return
	}

	if err := h.store.EnsureDefaultDataForUser(user.ID); err != nil {
		httputil.InternalError(c, err)
		return
	}

	h.recordLoginSuccess(limiterKey)
	h.respondWithToken(c, user)
}

func (h *Handler) me(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var user models.User
	if err := h.store.DB.First(&user, uid).Error; err != nil {
		httputil.NotFound(c, "user not found")
		return
	}
	c.JSON(http.StatusOK, currentUserFromModel(user))
}

func (h *Handler) refresh(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var user models.User
	if err := h.store.DB.First(&user, uid).Error; err != nil {
		httputil.NotFound(c, "user not found")
		return
	}
	if token, ok := middleware.BearerToken(c.GetHeader("Authorization")); ok {
		invalidToken, err := h.revokeTokenFromContext(c, token)
		if invalidToken {
			httputil.Unauthorized(c, "invalid token")
			return
		}
		if err != nil {
			httputil.InternalError(c, err)
			return
		}
	}
	h.respondWithToken(c, user)
}

func (h *Handler) logout(c *gin.Context) {
	token, ok := middleware.BearerToken(c.GetHeader("Authorization"))
	if !ok {
		httputil.Unauthorized(c, "missing Authorization header")
		return
	}
	invalidToken, err := h.revokeTokenFromContext(c, token)
	if invalidToken {
		httputil.Unauthorized(c, "invalid token")
		return
	}
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) EnsureBootstrapAdmin() error {
	username := strings.TrimSpace(h.admin.Username)
	password := strings.TrimSpace(h.admin.Password)
	name := strings.TrimSpace(h.admin.Name)
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

func (h *Handler) respondWithToken(c *gin.Context, user models.User) {
	if h.tokenService == nil {
		httputil.InternalError(c, errors.New("token service is not configured"))
		return
	}
	token, err := h.tokenService.BuildToken(user.ID)
	if err != nil {
		httputil.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token: token,
		User:  currentUserFromModel(user),
	})
}

func (h *Handler) tokenRevocationTTL(token string) (time.Duration, error) {
	if h.tokenService == nil {
		return 0, errors.New("token service is not configured")
	}
	expiresAt, err := h.tokenService.TokenRevocationExpiresAt(token)
	if err != nil {
		return 0, err
	}
	return time.Until(expiresAt), nil
}

func (h *Handler) revokeTokenFromContext(c *gin.Context, token string) (bool, error) {
	revoker := tokenRevokerFromContext(c)
	if revoker == nil {
		return false, nil
	}
	ttl, err := h.tokenRevocationTTL(token)
	if err != nil {
		return true, nil
	}
	if err := revoker.RevokeToken(c.Request.Context(), token, ttl); err != nil {
		return false, err
	}
	return false, nil
}

func (h *Handler) recordLoginFailure(key string) {
	if h.loginLimiter != nil {
		h.loginLimiter.RecordFailure(key)
	}
}

func (h *Handler) recordLoginSuccess(key string) {
	if h.loginLimiter != nil {
		h.loginLimiter.RecordSuccess(key)
	}
}

func newLoginLimiterFromConfig(cfg config.LoginRateLimitConfig) *loginLimiter {
	if cfg.MaxFailures <= 0 || cfg.Window <= 0 {
		return nil
	}
	return newLoginLimiter(cfg.MaxFailures, cfg.Window)
}

func tokenRevokerFromContext(c *gin.Context) *TokenRevoker {
	value, ok := c.Get("token_revoker")
	if !ok {
		return nil
	}
	revoker, _ := value.(*TokenRevoker)
	return revoker
}
