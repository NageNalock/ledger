package controller

import (
	"ledger/api/db"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type TrendRequest struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	Type      string `form:"type" binding:"required,oneof=day month"`
	ProjectID string `form:"project_id"`
}

type TrendItem struct {
	Date              string             `json:"date"`
	Amount            float64            `json:"amount"`
	CategoryBreakdown map[string]float64 `json:"breakdown"` // L1 -> Amount
}

type CategoryReportItem struct {
	Name     string               `json:"name"`
	Value    float64              `json:"value"`
	Children []CategoryReportItem `json:"children" gorm:"-"` // Ignore for GORM Scan
}

type MerchantReportItem struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// GetTrendReport returns spending trend over time with category breakdown
func GetTrendReport(c *gin.Context) {
	var req TrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var dateFormat string
	if req.Type == "month" {
		dateFormat = "%Y-%m"
	} else {
		dateFormat = "%Y-%m-%d"
	}

	// We need raw data grouped by date AND category to support stacked bar
	type RawTrend struct {
		Date         string
		CategoryName string
		Amount       float64
	}

	var rawResults []RawTrend

	// If StartDate/EndDate is "all", handle it?
	// Frontend will pass date range. If "all", start/end might be empty or specific values.
	// But req binding required start_date/end_date.
	// We assume frontend handles "all" by passing very wide range or backend handles empty?
	// The binding says required. So frontend MUST pass range.

	query := db.DB.Table("transactions").
		Select("strftime(?, transactions.transaction_time) as date, category_l1.name as category_name, SUM(transactions.amount) as amount", dateFormat).
		Joins("left join category_l1 on transactions.category_l1_id = category_l1.id").
		Where("transactions.transaction_time BETWEEN ? AND ?", req.StartDate+" 00:00:00", req.EndDate+" 23:59:59")

	if req.ProjectID != "" {
		query = query.Where("transactions.project_id = ?", req.ProjectID)
	}

	err := query.
		Group("date, category_name").
		Order("date").
		Scan(&rawResults).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Post-process to aggregate by date
	dateMap := make(map[string]*TrendItem)
	for _, r := range rawResults {
		if _, ok := dateMap[r.Date]; !ok {
			dateMap[r.Date] = &TrendItem{
				Date:              r.Date,
				Amount:            0,
				CategoryBreakdown: make(map[string]float64),
			}
		}
		item := dateMap[r.Date]
		item.Amount += r.Amount
		item.CategoryBreakdown[r.CategoryName] = r.Amount
	}

	// Convert map to slice
	var results []TrendItem
	for _, v := range dateMap {
		results = append(results, *v)
	}

	// Sort results by date? Map iteration is random.
	// Actually better to fill dates first then populate.

	filledResults := fillMissingDatesAndSort(results, req.StartDate, req.EndDate, req.Type)
	c.JSON(http.StatusOK, filledResults)
}

// GetCategoryReport returns spending distribution by L1 and L2
func GetCategoryReport(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}

	// Get L1 Stats
	var l1Results []CategoryReportItem
	err := db.DB.Table("transactions").
		Select("category_l1.name as name, SUM(transactions.amount) as value").
		Joins("left join category_l1 on transactions.category_l1_id = category_l1.id").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59").
		Group("category_l1.name").
		Order("value desc").
		Scan(&l1Results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get L2 Stats
	type L2Raw struct {
		L1Name string
		L2Name string
		Value  float64
	}
	var l2Raw []L2Raw
	db.DB.Table("transactions").
		Select("category_l1.name as l1_name, category_l2.name as l2_name, SUM(transactions.amount) as value").
		Joins("left join category_l1 on transactions.category_l1_id = category_l1.id").
		Joins("left join category_l2 on transactions.category_l2_id = category_l2.id").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59").
		Group("category_l1.name, category_l2.name").
		Scan(&l2Raw)

	// Merge L2 into L1
	for i := range l1Results {
		var children []CategoryReportItem
		for _, raw := range l2Raw {
			if raw.L1Name == l1Results[i].Name {
				children = append(children, CategoryReportItem{
					Name:  raw.L2Name,
					Value: raw.Value,
				})
			}
		}
		sort.Slice(children, func(a, b int) bool {
			return children[a].Value > children[b].Value
		})
		l1Results[i].Children = children
	}

	if l1Results == nil {
		l1Results = []CategoryReportItem{}
	}

	c.JSON(http.StatusOK, l1Results)
}

func GetMerchantReport(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	projectID := c.Query("project_id")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}

	limit := parseIntDefault(c.Query("limit"), 10)
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	var results []MerchantReportItem
	query := db.DB.Table("transactions").
		Select("CASE WHEN transactions.merchant IS NULL OR TRIM(transactions.merchant) = '' THEN '—' ELSE transactions.merchant END as name, SUM(transactions.amount) as value").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")

	if projectID != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}

	err := query.Group("name").Order("value desc").Limit(limit).Scan(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []MerchantReportItem{}
	}
	c.JSON(http.StatusOK, results)
}

func fillMissingDatesAndSort(data []TrendItem, start, end, freq string) []TrendItem {
	dataMap := make(map[string]TrendItem)
	for _, item := range data {
		dataMap[item.Date] = item
	}

	var result []TrendItem

	layout := "2006-01-02"
	if freq == "month" {
		layout = "2006-01"
		if len(start) > 7 {
			start = start[:7]
		}
		if len(end) > 7 {
			end = end[:7]
		}
	}

	currTime, _ := time.Parse(layout, start)
	endTime, _ := time.Parse(layout, end)

	for !currTime.After(endTime) {
		dateStr := currTime.Format(layout)
		if item, ok := dataMap[dateStr]; ok {
			result = append(result, item)
		} else {
			result = append(result, TrendItem{
				Date:              dateStr,
				Amount:            0,
				CategoryBreakdown: make(map[string]float64),
			})
		}

		if freq == "month" {
			currTime = currTime.AddDate(0, 1, 0)
		} else {
			currTime = currTime.AddDate(0, 0, 1)
		}
	}

	return result
}
