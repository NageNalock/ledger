package controller

import (
	"fmt"
	"ledger/api/db"
	"ledger/api/model"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TransactionInput struct {
	TransactionTime string  `json:"transaction_time" binding:"required"`
	Merchant        string  `json:"merchant"`
	Amount          float64 `json:"amount" binding:"required"`
	CategoryL1ID    uint    `json:"category_l1_id" binding:"required"`
	CategoryL2ID    uint    `json:"category_l2_id" binding:"required"`
	Source          string  `json:"source"`
	Remark          string  `json:"remark"`
	ProjectID       *uint   `json:"project_id"`
}

func CreateTransaction(c *gin.Context) {
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !validateTransactionReferences(c, input) {
		return
	}

	tTime, err := time.Parse(time.RFC3339, input.TransactionTime) // Expecting ISO 8601 / RFC3339
	if err != nil {
		// Try RFC3339 with space instead of T
		tTime, err = time.Parse("2006-01-02 15:04:05Z07:00", input.TransactionTime)
		if err != nil {
			// Fallback to old formats just in case
			tTime, err = time.ParseInLocation("2006-01-02 15:04:05", input.TransactionTime, time.Local)
			if err != nil {
				tTime, err = time.ParseInLocation("2006-01-02", input.TransactionTime, time.Local)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":       "Invalid time format",
						"debug_input": input.TransactionTime,
						"debug_err":   err.Error(),
					})
					return
				}
			}
		}
	}

	tx := model.Transaction{
		UserID:          userID.(uint),
		TransactionTime: tTime,
		Merchant:        input.Merchant,
		Amount:          input.Amount,
		CategoryL1ID:    input.CategoryL1ID,
		CategoryL2ID:    input.CategoryL2ID,
		Source:          input.Source,
		Remark:          input.Remark,
		ProjectID:       input.ProjectID,
		ExternalID:      "manual-" + uuid.NewString(),
	}

	if err := db.DB.Create(&tx).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func UpdateTransaction(c *gin.Context) {
	id := c.Param("id")
	var input TransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validateTransactionReferences(c, input) {
		return
	}

	tTime, err := time.Parse(time.RFC3339, input.TransactionTime) // Expecting ISO 8601 / RFC3339
	if err != nil {
		// Try RFC3339 with space instead of T
		tTime, err = time.Parse("2006-01-02 15:04:05Z07:00", input.TransactionTime)
		if err != nil {
			// Fallback to old formats just in case
			tTime, err = time.ParseInLocation("2006-01-02 15:04:05", input.TransactionTime, time.Local)
			if err != nil {
				tTime, err = time.ParseInLocation("2006-01-02", input.TransactionTime, time.Local)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":       "Invalid time format",
						"debug_input": input.TransactionTime,
						"debug_err":   err.Error(),
					})
					return
				}
			}
		}
	}

	updates := map[string]any{
		"transaction_time": tTime,
		"merchant":         input.Merchant,
		"amount":           input.Amount,
		"category_l1_id":   input.CategoryL1ID,
		"category_l2_id":   input.CategoryL2ID,
		"source":           input.Source,
		"remark":           input.Remark,
		"project_id":       input.ProjectID,
	}

	res := transactionQuery(c).Model(&model.Transaction{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	var tx model.Transaction
	if err := db.DB.Preload("User").Preload("CategoryL1").Preload("CategoryL2").Preload("Project").Where("id = ?", id).First(&tx).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction reload failed"})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func DeleteTransaction(c *gin.Context) {
	id := c.Param("id")
	res := transactionQuery(c).Where("id = ?", id).Delete(&model.Transaction{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction deleted"})
}

func GetTransactions(c *gin.Context) {
	orderClause := "transaction_time desc"
	if sortBy := c.Query("sort_by"); sortBy == "amount" || sortBy == "transaction_time" || sortBy == "source" {
		order := "desc"
		if sortOrder := c.Query("sort_order"); sortOrder == "asc" || sortOrder == "desc" {
			order = sortOrder
		}
		orderClause = sortBy + " " + order
	}

	query := transactionQuery(c).Preload("User").Preload("CategoryL1").Preload("CategoryL2").Preload("Project").Order(orderClause)
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("transactions.user_id = ?", userID)
	}

	// Filters
	if year := c.Query("year"); year != "" {
		query = query.Where("strftime('%Y', transaction_time) = ?", year)
	}
	if month := c.Query("month"); month != "" {
		if len(month) == 1 {
			month = "0" + month
		}
		query = query.Where("strftime('%m', transaction_time) = ?", month)
	}
	if startDate := c.Query("start_date"); startDate != "" {
		// Append time to make it inclusive if user only sends date
		if len(startDate) == 10 { // YYYY-MM-DD
			query = query.Where("transaction_time >= ?", startDate+" 00:00:00")
		} else {
			query = query.Where("transaction_time >= ?", startDate)
		}
	}
	if endDate := c.Query("end_date"); endDate != "" {
		if len(endDate) == 10 {
			query = query.Where("transaction_time <= ?", endDate+" 23:59:59")
		} else {
			query = query.Where("transaction_time <= ?", endDate)
		}
	}

	// Category Filter (by Name for simplicity with Charts)
	if catL1 := c.Query("category_l1"); catL1 != "" {
		query = query.Joins("JOIN category_l1 ON transactions.category_l1_id = category_l1.id").
			Where("category_l1.name = ?", catL1)
	}
	if catL1ID := c.Query("category_l1_id"); catL1ID != "" {
		query = query.Where("transactions.category_l1_id = ?", catL1ID)
	}
	if catL2ID := c.Query("category_l2_id"); catL2ID != "" {
		query = query.Where("transactions.category_l2_id = ?", catL2ID)
	}
	if projectID := c.Query("project_id"); projectID != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	if source := c.Query("source"); source != "" {
		query = query.Where("transactions.source = ?", source)
	}

	// Pagination
	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := c.Query("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var total int64
	query.Model(&model.Transaction{}).Count(&total)

	offset := (page - 1) * pageSize
	var transactions []model.Transaction
	if err := query.Limit(pageSize).Offset(offset).Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      transactions,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Project Handlers

func GetProjects(c *gin.Context) {
	var projects []model.Project
	db.DB.Order("created_at desc").Find(&projects)
	c.JSON(http.StatusOK, projects)
}

func GetProject(c *gin.Context) {
	id := c.Param("id")
	var project model.Project
	if err := db.DB.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}

func GetProjectSummary(c *gin.Context) {
	id := c.Param("id")
	var project model.Project
	if err := db.DB.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	query := db.DB.Model(&model.Transaction{}).Where("project_id = ?", id)

	if startDate := c.Query("start_date"); startDate != "" {
		if len(startDate) == 10 {
			query = query.Where("transaction_time >= ?", startDate+" 00:00:00")
		} else {
			query = query.Where("transaction_time >= ?", startDate)
		}
	}
	if endDate := c.Query("end_date"); endDate != "" {
		if len(endDate) == 10 {
			query = query.Where("transaction_time <= ?", endDate+" 23:59:59")
		} else {
			query = query.Where("transaction_time <= ?", endDate)
		}
	}
	if catL1ID := c.Query("category_l1_id"); catL1ID != "" {
		query = query.Where("category_l1_id = ?", catL1ID)
	}
	if catL2ID := c.Query("category_l2_id"); catL2ID != "" {
		query = query.Where("category_l2_id = ?", catL2ID)
	}
	if source := c.Query("source"); source != "" {
		query = query.Where("source = ?", source)
	}
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	var totalAmount float64
	if err := query.Select("COALESCE(SUM(amount), 0)").Scan(&totalAmount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project":      project,
		"total_amount": totalAmount,
	})
}

func CreateProject(c *gin.Context) {
	var input struct {
		Name          string  `json:"name" binding:"required"`
		Description   string  `json:"description"`
		BudgetEnabled bool    `json:"budget_enabled"`
		BudgetLimit   float64 `json:"budget_limit"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := model.Project{
		Name:          input.Name,
		Description:   input.Description,
		BudgetEnabled: input.BudgetEnabled,
		BudgetLimit:   input.BudgetLimit,
	}

	if err := db.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func UpdateProject(c *gin.Context) {
	id := c.Param("id")
	var project model.Project
	if err := db.DB.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	var input struct {
		Name          string   `json:"name" binding:"required"`
		Description   string   `json:"description"`
		BudgetEnabled *bool    `json:"budget_enabled"`
		BudgetLimit   *float64 `json:"budget_limit"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project.Name = input.Name
	project.Description = input.Description
	if input.BudgetEnabled != nil {
		project.BudgetEnabled = *input.BudgetEnabled
	}
	if input.BudgetLimit != nil {
		project.BudgetLimit = *input.BudgetLimit
	}

	if err := db.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

func DeleteProject(c *gin.Context) {
	id := c.Param("id")
	// Check usage? Maybe set transactions to null project
	db.DB.Model(&model.Transaction{}).Where("project_id = ?", id).Update("project_id", nil)

	if err := db.DB.Delete(&model.Project{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Project deleted"})
}
