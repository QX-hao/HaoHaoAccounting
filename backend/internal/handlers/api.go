package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sealos/haohaoaccounting/backend/internal/cache"
	"github.com/sealos/haohaoaccounting/backend/internal/middleware"
	"github.com/sealos/haohaoaccounting/backend/internal/models"
	"github.com/sealos/haohaoaccounting/backend/internal/services"
	"github.com/sealos/haohaoaccounting/backend/internal/store"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type Handler struct {
	store *store.Store
	cache *cache.RedisCache
}

func New(s *store.Store, redisCache *cache.RedisCache) *Handler {
	return &Handler{store: s, cache: redisCache}
}

func (h *Handler) Register(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "ok",
			"redisEnabled": h.cache != nil && h.cache.Enabled(),
		})
	})

	api := engine.Group("/api/v1")
	{
		api.POST("/auth/login", h.login)
	}

	authGroup := api.Group("")
	authGroup.Use(middleware.RequireAuth())
	{
		authGroup.GET("/me", h.me)
		authGroup.GET("/accounts", h.listAccounts)
		authGroup.POST("/accounts", h.createAccount)
		authGroup.PUT("/accounts/:id", h.updateAccount)
		authGroup.DELETE("/accounts/:id", h.deleteAccount)

		authGroup.GET("/categories", h.listCategories)
		authGroup.POST("/categories", h.createCategory)
		authGroup.PUT("/categories/:id", h.updateCategory)
		authGroup.DELETE("/categories/:id", h.deleteCategory)

		authGroup.GET("/transactions", h.listTransactions)
		authGroup.POST("/transactions", h.createTransaction)
		authGroup.PUT("/transactions/:id", h.updateTransaction)
		authGroup.DELETE("/transactions/:id", h.deleteTransaction)

		authGroup.POST("/ai/parse", h.aiParse)

		authGroup.GET("/reports/summary", h.reportSummary)

		authGroup.POST("/io/import", h.importData)
		authGroup.GET("/io/export", h.exportData)
	}
}

type loginRequest struct {
	LoginType  string `json:"loginType"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.LoginType = strings.ToLower(strings.TrimSpace(req.LoginType))
	req.Identifier = strings.TrimSpace(req.Identifier)
	req.Name = strings.TrimSpace(req.Name)

	if req.Identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifier is required"})
		return
	}

	var user models.User
	db := h.store.DB

	switch req.LoginType {
	case "phone":
		err := db.Where("phone = ?", req.Identifier).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{Phone: req.Identifier, Name: fallbackName(req.Name, "手机用户")}
			if err := db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case "email":
		err := db.Where("email = ?", req.Identifier).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{Email: req.Identifier, Name: fallbackName(req.Name, "邮箱用户")}
			if err := db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case "wechat":
		err := db.Where("wechat_id = ?", req.Identifier).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{WechatID: req.Identifier, Name: fallbackName(req.Name, "微信用户")}
			if err := db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported loginType, use phone/email/wechat"})
		return
	}

	if err := h.store.EnsureDefaultDataForUser(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": middleware.BuildToken(user.ID),
		"user":  user,
	})
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

type accountRequest struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Balance float64 `json:"balance"`
}

func (h *Handler) listAccounts(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var accounts []models.Account
	if err := h.store.DB.Where("user_id = ?", uid).Order("id asc").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

func (h *Handler) createAccount(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	account := models.Account{
		UserID:  uid,
		Name:    strings.TrimSpace(req.Name),
		Type:    fallbackName(strings.TrimSpace(req.Type), "custom"),
		Balance: req.Balance,
	}
	if account.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if err := h.store.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusCreated, account)
}

func (h *Handler) updateAccount(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var account models.Account
	if err := h.store.DB.Where("id = ? AND user_id = ?", id, uid).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Name) != "" {
		account.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Type) != "" {
		account.Type = strings.TrimSpace(req.Type)
	}
	account.Balance = req.Balance

	if err := h.store.DB.Save(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, account)
}

func (h *Handler) deleteAccount(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var count int64
	if err := h.store.DB.Model(&models.Transaction{}).Where("user_id = ? AND account_id = ?", uid, id).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account in use by transactions"})
		return
	}

	if err := h.store.DB.Where("id = ? AND user_id = ?", id, uid).Delete(&models.Account{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type categoryRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *Handler) listCategories(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	txType := strings.TrimSpace(c.Query("type"))

	query := h.store.DB.Model(&models.Category{}).
		Where("is_system = ? OR user_id = ?", true, uid)
	if txType != "" {
		query = query.Where("type = ?", txType)
	}

	var categories []models.Category
	if err := query.Order("is_system desc, id asc").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *Handler) createCategory(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "income" && req.Type != "expense" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be income or expense"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	category := models.Category{UserID: &uid, Name: strings.TrimSpace(req.Name), Type: req.Type, IsSystem: false}
	if err := h.store.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusCreated, category)
}

func (h *Handler) updateCategory(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var category models.Category
	if err := h.store.DB.Where("id = ?", id).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	if category.IsSystem || category.UserID == nil || *category.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "system category cannot be modified"})
		return
	}

	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Name) != "" {
		category.Name = strings.TrimSpace(req.Name)
	}
	if t := strings.ToLower(strings.TrimSpace(req.Type)); t == "income" || t == "expense" {
		category.Type = t
	}

	if err := h.store.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, category)
}

func (h *Handler) deleteCategory(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var category models.Category
	if err := h.store.DB.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	if category.IsSystem || category.UserID == nil || *category.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "system category cannot be deleted"})
		return
	}

	var count int64
	if err := h.store.DB.Model(&models.Transaction{}).Where("user_id = ? AND category_id = ?", uid, id).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category in use by transactions"})
		return
	}

	if err := h.store.DB.Delete(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type transactionRequest struct {
	Type       string    `json:"type"`
	Amount     float64   `json:"amount"`
	CategoryID uint      `json:"categoryId"`
	AccountID  uint      `json:"accountId"`
	Note       string    `json:"note"`
	Tags       []string  `json:"tags"`
	OccurredAt time.Time `json:"occurredAt"`
	Source     string    `json:"source"`
}

func (h *Handler) listTransactions(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)

	page := parseIntQuery(c.Query("page"), 1)
	pageSize := parseIntQuery(c.Query("pageSize"), 20)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	query := h.store.DB.Model(&models.Transaction{}).Where("user_id = ?", uid)
	if start := strings.TrimSpace(c.Query("start")); start != "" {
		if t, err := parseDateTime(start); err == nil {
			query = query.Where("occurred_at >= ?", t)
		}
	}
	if end := strings.TrimSpace(c.Query("end")); end != "" {
		if t, err := parseDateTime(end); err == nil {
			query = query.Where("occurred_at <= ?", t)
		}
	}
	if t := strings.TrimSpace(c.Query("type")); t != "" {
		query = query.Where("type = ?", t)
	}
	if cid := parseUintQuery(c.Query("categoryId")); cid > 0 {
		query = query.Where("category_id = ?", cid)
	}
	if aid := parseUintQuery(c.Query("accountId")); aid > 0 {
		query = query.Where("account_id = ?", aid)
	}
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("note LIKE ? OR tags LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rows []models.Transaction
	if err := query.Preload("Category").Preload("Account").
		Order("occurred_at desc, id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"pagination": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h *Handler) createTransaction(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	var req transactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := validateTransactionRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.ensureCategoryAndAccount(uid, req.Type, req.CategoryID, req.AccountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now()
	}

	tx := models.Transaction{
		UserID:     uid,
		Type:       req.Type,
		Amount:     req.Amount,
		CategoryID: req.CategoryID,
		AccountID:  req.AccountID,
		Note:       strings.TrimSpace(req.Note),
		Tags:       strings.Join(req.Tags, ","),
		OccurredAt: req.OccurredAt,
		Source:     fallbackName(strings.TrimSpace(req.Source), "manual"),
	}

	if err := h.store.DB.Transaction(func(dbtx *gorm.DB) error {
		if err := dbtx.Create(&tx).Error; err != nil {
			return err
		}
		return applyAccountDelta(dbtx, tx.AccountID, tx.Type, tx.Amount)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.store.DB.Preload("Category").Preload("Account").First(&tx, tx.ID)
	h.invalidateUserCache(uid)
	c.JSON(http.StatusCreated, tx)
}

func (h *Handler) updateTransaction(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var existing models.Transaction
	if err := h.store.DB.Where("id = ? AND user_id = ?", id, uid).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	var req transactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := validateTransactionRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.ensureCategoryAndAccount(uid, req.Type, req.CategoryID, req.AccountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = existing.OccurredAt
	}

	updated := existing
	updated.Type = req.Type
	updated.Amount = req.Amount
	updated.CategoryID = req.CategoryID
	updated.AccountID = req.AccountID
	updated.Note = strings.TrimSpace(req.Note)
	updated.Tags = strings.Join(req.Tags, ",")
	updated.OccurredAt = req.OccurredAt
	updated.Source = fallbackName(strings.TrimSpace(req.Source), existing.Source)

	if err := h.store.DB.Transaction(func(dbtx *gorm.DB) error {
		if err := revertAccountDelta(dbtx, existing.AccountID, existing.Type, existing.Amount); err != nil {
			return err
		}
		if err := applyAccountDelta(dbtx, updated.AccountID, updated.Type, updated.Amount); err != nil {
			return err
		}
		return dbtx.Save(&updated).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.store.DB.Preload("Category").Preload("Account").First(&updated, updated.ID)
	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) deleteTransaction(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	id, _ := strconv.Atoi(c.Param("id"))

	var tx models.Transaction
	if err := h.store.DB.Where("id = ? AND user_id = ?", id, uid).First(&tx).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	if err := h.store.DB.Transaction(func(dbtx *gorm.DB) error {
		if err := revertAccountDelta(dbtx, tx.AccountID, tx.Type, tx.Amount); err != nil {
			return err
		}
		return dbtx.Delete(&tx).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.invalidateUserCache(uid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type aiParseRequest struct {
	Text string `json:"text"`
}

func (h *Handler) aiParse(c *gin.Context) {
	var req aiParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	uid := middleware.UserIDFromContext(c)
	cacheKey := cache.UserAIParseKey(uid, req.Text)

	if h.cache != nil && h.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		var cached services.AIParseResult
		ok, err := h.cache.GetJSON(ctx, cacheKey, &cached)
		if err == nil && ok {
			c.JSON(http.StatusOK, gin.H{
				"requiresConfirmation": true,
				"cached":               true,
				"result":               cached,
			})
			return
		}
	}

	parsed := services.ParseNaturalLedgerText(req.Text)
	if h.cache != nil && h.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.cache.SetJSON(ctx, cacheKey, parsed, 10*time.Minute)
	}

	c.JSON(http.StatusOK, gin.H{
		"requiresConfirmation": true,
		"cached":               false,
		"result":               parsed,
	})
}

type categoryStat struct {
	CategoryID uint    `json:"categoryId"`
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
}

type accountStat struct {
	AccountID uint    `json:"accountId"`
	Account   string  `json:"account"`
	Amount    float64 `json:"amount"`
}

func (h *Handler) reportSummary(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	start, end := resolveRange(c)
	cacheKey := cache.UserReportKey(uid, start, end)

	if h.cache != nil && h.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var cached map[string]any
		ok, err := h.cache.GetJSON(ctx, cacheKey, &cached)
		if err == nil && ok {
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	var rows []models.Transaction
	if err := h.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", uid, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	summary := gin.H{
		"start": start,
		"end":   end,
	}

	income := 0.0
	expense := 0.0
	categoryMap := map[uint]*categoryStat{}
	accountMap := map[uint]*accountStat{}
	monthly := map[string]gin.H{}

	for _, row := range rows {
		if row.Type == "income" {
			income += row.Amount
		} else {
			expense += row.Amount
		}

		if row.Type == "expense" {
			v, ok := categoryMap[row.CategoryID]
			if !ok {
				v = &categoryStat{CategoryID: row.CategoryID, Category: row.Category.Name}
				categoryMap[row.CategoryID] = v
			}
			v.Amount += row.Amount
		}

		if row.Type == "expense" {
			v, ok := accountMap[row.AccountID]
			if !ok {
				v = &accountStat{AccountID: row.AccountID, Account: row.Account.Name}
				accountMap[row.AccountID] = v
			}
			v.Amount += row.Amount
		}

		monthKey := row.OccurredAt.Format("2006-01")
		if _, ok := monthly[monthKey]; !ok {
			monthly[monthKey] = gin.H{"month": monthKey, "income": 0.0, "expense": 0.0}
		}
		if row.Type == "income" {
			monthly[monthKey]["income"] = monthly[monthKey]["income"].(float64) + row.Amount
		} else {
			monthly[monthKey]["expense"] = monthly[monthKey]["expense"].(float64) + row.Amount
		}
	}

	categoryList := make([]categoryStat, 0, len(categoryMap))
	for _, v := range categoryMap {
		categoryList = append(categoryList, *v)
	}
	sort.Slice(categoryList, func(i, j int) bool { return categoryList[i].Amount > categoryList[j].Amount })

	accountList := make([]accountStat, 0, len(accountMap))
	for _, v := range accountMap {
		accountList = append(accountList, *v)
	}
	sort.Slice(accountList, func(i, j int) bool { return accountList[i].Amount > accountList[j].Amount })

	monthKeys := make([]string, 0, len(monthly))
	for k := range monthly {
		monthKeys = append(monthKeys, k)
	}
	sort.Strings(monthKeys)
	monthlyTrend := make([]gin.H, 0, len(monthKeys))
	for _, k := range monthKeys {
		monthlyTrend = append(monthlyTrend, monthly[k])
	}

	prevStart := start.Add(-end.Sub(start) - time.Second)
	prevEnd := start.Add(-time.Second)
	prevIncome, prevExpense := h.sumIncomeExpense(uid, prevStart, prevEnd)

	summary["income"] = income
	summary["expense"] = expense
	summary["balance"] = income - expense
	summary["byCategory"] = categoryList
	summary["byAccount"] = accountList
	summary["monthlyTrend"] = monthlyTrend
	summary["periodCompare"] = gin.H{
		"current":  gin.H{"income": income, "expense": expense},
		"previous": gin.H{"income": prevIncome, "expense": prevExpense},
	}

	if h.cache != nil && h.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.cache.SetJSON(ctx, cacheKey, summary, 2*time.Minute)
	}

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) sumIncomeExpense(userID uint, start, end time.Time) (float64, float64) {
	var rows []models.Transaction
	if err := h.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", userID, start, end).
		Select("type, amount").
		Find(&rows).Error; err != nil {
		return 0, 0
	}
	income, expense := 0.0, 0.0
	for _, row := range rows {
		if row.Type == "income" {
			income += row.Amount
		} else {
			expense += row.Amount
		}
	}
	return income, expense
}

func (h *Handler) exportData(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	start, end := resolveRange(c)

	var rows []models.Transaction
	if err := h.store.DB.Where("user_id = ? AND occurred_at >= ? AND occurred_at <= ?", uid, start, end).
		Preload("Category").Preload("Account").
		Order("occurred_at asc").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if format == "xlsx" {
		h.exportXLSX(c, rows)
		return
	}
	h.exportCSV(c, rows)
}

func (h *Handler) exportCSV(c *gin.Context, rows []models.Transaction) {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".csv"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	_ = writer.Write([]string{"occurred_at", "type", "amount", "category", "account", "note", "tags", "source"})
	for _, row := range rows {
		_ = writer.Write([]string{
			row.OccurredAt.Format(time.RFC3339),
			row.Type,
			strconv.FormatFloat(row.Amount, 'f', 2, 64),
			row.Category.Name,
			row.Account.Name,
			row.Note,
			row.Tags,
			row.Source,
		})
	}
}

func (h *Handler) exportXLSX(c *gin.Context, rows []models.Transaction) {
	filename := "transactions_" + time.Now().Format("20060102150405") + ".xlsx"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"occurred_at", "type", "amount", "category", "account", "note", "tags", "source"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	for idx, row := range rows {
		line := idx + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", line), row.OccurredAt.Format(time.RFC3339))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), row.Type)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", line), row.Amount)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", line), row.Category.Name)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", line), row.Account.Name)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", line), row.Note)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", line), row.Tags)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", line), row.Source)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, _ = c.Writer.Write(buf.Bytes())
}

func (h *Handler) importData(c *gin.Context) {
	uid := middleware.UserIDFromContext(c)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	sourceRows, err := readImportRows(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success := 0
	failed := 0
	errorsList := make([]string, 0)

	for i, row := range sourceRows {
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		record, err := parseImportRecord(row)
		if err != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", i+2, err))
			continue
		}

		category, err := h.store.FindOrCreateCategory(uid, record.Type, record.Category)
		if err != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", i+2, err))
			continue
		}
		account, err := h.store.FindOrCreateAccount(uid, record.Account)
		if err != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", i+2, err))
			continue
		}

		entry := models.Transaction{
			UserID:     uid,
			Type:       record.Type,
			Amount:     record.Amount,
			CategoryID: category.ID,
			AccountID:  account.ID,
			Note:       record.Note,
			Tags:       record.Tags,
			Source:     "import",
			OccurredAt: record.OccurredAt,
		}
		if err := h.store.DB.Transaction(func(dbtx *gorm.DB) error {
			if err := dbtx.Create(&entry).Error; err != nil {
				return err
			}
			return applyAccountDelta(dbtx, entry.AccountID, entry.Type, entry.Amount)
		}); err != nil {
			failed++
			errorsList = append(errorsList, fmt.Sprintf("line %d: %v", i+2, err))
			continue
		}

		success++
	}

	h.invalidateUserCache(uid)

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"failed":  failed,
		"errors":  errorsList,
	})
}

type importRecord struct {
	OccurredAt time.Time
	Type       string
	Amount     float64
	Category   string
	Account    string
	Note       string
	Tags       string
}

func readImportRows(file *multipart.FileHeader) ([][]string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if ext == ".xlsx" {
		tmp, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		xlsx, err := excelize.OpenReader(bytes.NewReader(tmp))
		if err != nil {
			return nil, err
		}
		sheet := xlsx.GetSheetName(0)
		rows, err := xlsx.GetRows(sheet)
		if err != nil {
			return nil, err
		}
		if len(rows) <= 1 {
			return nil, errors.New("empty xlsx")
		}
		return rows[1:], nil
	}

	reader := csv.NewReader(f)
	all, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(all) <= 1 {
		return nil, errors.New("empty csv")
	}
	return all[1:], nil
}

func (h *Handler) ensureCategoryAndAccount(userID uint, txType string, categoryID, accountID uint) error {
	var category models.Category
	if err := h.store.DB.Where("id = ?", categoryID).First(&category).Error; err != nil {
		return errors.New("category not found")
	}
	if category.Type != txType {
		return errors.New("category type mismatch")
	}
	if !category.IsSystem {
		if category.UserID == nil || *category.UserID != userID {
			return errors.New("category not accessible")
		}
	}

	var account models.Account
	if err := h.store.DB.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return errors.New("account not found")
	}
	return nil
}

func (h *Handler) invalidateUserCache(userID uint) {
	if h.cache == nil || !h.cache.Enabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.cache.DeleteByPrefix(ctx, cache.UserReportPrefix(userID))
}

func parseImportRecord(row []string) (importRecord, error) {
	get := func(i int) string {
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	occurredAt, err := parseDateTime(get(0))
	if err != nil {
		return importRecord{}, fmt.Errorf("invalid occurred_at")
	}
	typeVal := strings.ToLower(get(1))
	if typeVal != "income" && typeVal != "expense" {
		return importRecord{}, fmt.Errorf("invalid type")
	}
	amount, err := strconv.ParseFloat(get(2), 64)
	if err != nil || amount <= 0 {
		return importRecord{}, fmt.Errorf("invalid amount")
	}
	category := fallbackName(get(3), "餐饮")
	account := fallbackName(get(4), "现金")

	return importRecord{
		OccurredAt: occurredAt,
		Type:       typeVal,
		Amount:     amount,
		Category:   category,
		Account:    account,
		Note:       get(5),
		Tags:       get(6),
	}, nil
}

func resolveRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now

	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := parseDateTime(raw); err == nil {
			start = t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := parseDateTime(raw); err == nil {
			end = t
		}
	}
	return start, end
}

func parseDateTime(raw string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006/01/02"}
	for _, format := range formats {
		if t, err := time.Parse(format, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid datetime")
}

func validateTransactionRequest(req transactionRequest) error {
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.Type != "income" && req.Type != "expense" {
		return errors.New("type must be income or expense")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be > 0")
	}
	if req.CategoryID == 0 {
		return errors.New("categoryId is required")
	}
	if req.AccountID == 0 {
		return errors.New("accountId is required")
	}
	if strings.TrimSpace(req.Note) == "" {
		return errors.New("note is required")
	}
	return nil
}

func applyAccountDelta(dbtx *gorm.DB, accountID uint, txType string, amount float64) error {
	delta := amount
	if txType == "expense" {
		delta = -amount
	}
	return dbtx.Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("balance", gorm.Expr("balance + ?", delta)).Error
}

func revertAccountDelta(dbtx *gorm.DB, accountID uint, txType string, amount float64) error {
	delta := amount
	if txType == "expense" {
		delta = -amount
	}
	return dbtx.Model(&models.Account{}).
		Where("id = ?", accountID).
		Update("balance", gorm.Expr("balance - ?", delta)).Error
}

func fallbackName(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func parseIntQuery(v string, fallback int) int {
	i, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return i
}

func parseUintQuery(v string) uint {
	u, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return uint(u)
}
