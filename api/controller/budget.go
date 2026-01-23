package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"ledger/api/db"
	"ledger/api/model"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type budgetThresholdDTO struct {
	Pct   float64 `json:"pct"`
	Label string  `json:"label"`
}

type budgetLineInput struct {
	CategoryL1ID *uint   `json:"category_l1_id"`
	CategoryL2ID *uint   `json:"category_l2_id"`
	Amount       float64 `json:"amount"`
	Note         string  `json:"note"`
}

type budgetUpsertInput struct {
	Month       string               `json:"month" binding:"required"`
	Status      string               `json:"status"`
	TotalAmount float64              `json:"total_amount"`
	Thresholds  []budgetThresholdDTO `json:"thresholds"`
	Lines       []budgetLineInput    `json:"lines"`
}

type budgetLineUsageDTO struct {
	ID           uint              `json:"id"`
	Scope        string            `json:"scope"`
	CategoryL1ID *uint             `json:"category_l1_id,omitempty"`
	CategoryL2ID *uint             `json:"category_l2_id,omitempty"`
	CategoryL1   *model.CategoryL1 `json:"category_l1,omitempty"`
	CategoryL2   *model.CategoryL2 `json:"category_l2,omitempty"`
	Amount       float64           `json:"amount"`
	UsedAmount   float64           `json:"used_amount"`
	Remaining    float64           `json:"remaining"`
	UsageRatio   float64           `json:"usage_ratio"`
	Note         string            `json:"note"`
}

type budgetUsageDTO struct {
	BudgetID    string               `json:"budget_id"`
	Month       string               `json:"month"`
	Status      string               `json:"status"`
	TotalAmount float64              `json:"total_amount"`
	UsedAmount  float64              `json:"used_amount"`
	Remaining   float64              `json:"remaining"`
	UsageRatio  float64              `json:"usage_ratio"`
	Thresholds  []budgetThresholdDTO `json:"thresholds"`
	Waterline   string               `json:"waterline"`
	Lines       []budgetLineUsageDTO `json:"lines"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
	DiscardedAt *string              `json:"discarded_at,omitempty"`
}

func computeMonthSpend(month string) (float64, error) {
	start, end, err := monthRange(month)
	if err != nil {
		return 0, err
	}
	var spent float64
	if err := db.DB.Model(&model.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("transaction_time BETWEEN ? AND ?", start, end).
		Scan(&spent).Error; err != nil {
		return 0, err
	}
	return round2(spent), nil
}

func normalizeMonth(month string) (string, error) {
	m := strings.TrimSpace(month)
	if m == "" {
		return "", fmt.Errorf("month required")
	}
	if len(m) >= 7 {
		m = m[:7]
	}
	_, err := time.Parse("2006-01", m)
	if err != nil {
		return "", fmt.Errorf("invalid month")
	}
	return m, nil
}

func monthRange(month string) (time.Time, time.Time, error) {
	m, err := normalizeMonth(month)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01", m, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid month")
	}
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return start, end, nil
}

func evalWaterline(ratio float64, thresholds []budgetThresholdDTO) string {
	if len(thresholds) == 0 {
		return "ok"
	}
	sorted := make([]budgetThresholdDTO, 0, len(thresholds))
	for _, t := range thresholds {
		sorted = append(sorted, t)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Pct < sorted[j].Pct })
	label := "ok"
	for _, t := range sorted {
		if ratio >= t.Pct {
			if strings.TrimSpace(t.Label) != "" {
				label = strings.TrimSpace(t.Label)
			}
		}
	}
	return label
}

func defaultThresholdsIfEmpty(thresholds []budgetThresholdDTO) []budgetThresholdDTO {
	if len(thresholds) != 0 {
		return thresholds
	}
	return []budgetThresholdDTO{{Pct: 0.5, Label: "提醒"}, {Pct: 0.8, Label: "预警"}, {Pct: 1.0, Label: "超支"}}
}

func GetActiveBudget(c *gin.Context) {
	month := c.Query("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	m, err := normalizeMonth(month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
		return
	}

	spent, err := computeMonthSpend(m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var b model.Budget
	err = db.DB.Where("month = ? AND status = ?", m, "active").First(&b).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"budget": nil, "spent_amount": spent})
		return
	}

	usage, err := buildBudgetUsageDTO(b.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"budget": usage, "spent_amount": spent})
}

func ListBudgetHistory(c *gin.Context) {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if v := c.Query("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	month := strings.TrimSpace(c.Query("month"))
	status := strings.TrimSpace(c.Query("status"))

	query := db.DB.Model(&model.Budget{})
	if month != "" {
		m, err := normalizeMonth(month)
		if err == nil {
			query = query.Where("month = ?", m)
		}
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var budgets []model.Budget
	if err := query.Order("month desc").Order("created_at desc").Limit(limit).Offset(offset).Find(&budgets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]budgetUsageDTO, 0, len(budgets))
	for _, b := range budgets {
		usage, err := buildBudgetUsageDTO(b.ID)
		if err != nil {
			continue
		}
		items = append(items, usage)
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func CreateBudget(c *gin.Context) {
	var input budgetUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := normalizeMonth(input.Month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
		return
	}
	status := strings.TrimSpace(input.Status)
	if status != "active" {
		status = "draft"
	}

	thresholdBs, _ := json.Marshal(defaultThresholdsIfEmpty(input.Thresholds))
	now := time.Now()
	b := model.Budget{
		ID:          uuid.NewString(),
		Month:       m,
		Status:      status,
		TotalAmount: round2(input.TotalAmount),
		Thresholds:  string(thresholdBs),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := db.DB.Create(&b).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := upsertBudgetLines(b.ID, input.Lines, b.TotalAmount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if status == "active" {
		if err := activateBudget(b.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	usage, err := buildBudgetUsageDTO(b.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, usage)
}

func UpsertActiveBudget(c *gin.Context) {
	if _, exists := c.Get("userID"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input budgetUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	m, err := normalizeMonth(input.Month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
		return
	}
	amt := round2(input.TotalAmount)
	if amt < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid total_amount"})
		return
	}

	thresholdBs, _ := json.Marshal(defaultThresholdsIfEmpty(input.Thresholds))
	now := time.Now()

	var existing model.Budget
	err = db.DB.Where("month = ? AND status = ?", m, "active").First(&existing).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		b := model.Budget{
			ID:          uuid.NewString(),
			Month:       m,
			Status:      "active",
			TotalAmount: amt,
			Thresholds:  string(thresholdBs),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.DB.Create(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := upsertBudgetLines(b.ID, input.Lines, b.TotalAmount); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := activateBudget(b.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		usage, err := buildBudgetUsageDTO(b.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, usage)
		return
	}

	existing.TotalAmount = amt
	existing.Thresholds = string(thresholdBs)
	existing.UpdatedAt = now
	if err := db.DB.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := upsertBudgetLines(existing.ID, input.Lines, existing.TotalAmount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	usage, err := buildBudgetUsageDTO(existing.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, usage)
}

func DiscardActiveBudget(c *gin.Context) {
	if _, exists := c.Get("userID"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		var body struct {
			Month string `json:"month"`
		}
		_ = c.ShouldBindJSON(&body)
		month = strings.TrimSpace(body.Month)
	}
	if month == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month required"})
		return
	}
	m, err := normalizeMonth(month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
		return
	}

	var b model.Budget
	if err := db.DB.Where("month = ? AND status = ?", m, "active").First(&b).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Budget not found"})
		return
	}

	now := time.Now()
	if err := db.DB.Model(&model.Budget{}).Where("id = ?", b.ID).Updates(map[string]any{"status": "discarded", "discarded_at": &now, "updated_at": now}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func upsertBudgetLines(budgetID string, lines []budgetLineInput, totalAmount float64) error {
	cleanL1 := make([]budgetLineInput, 0, len(lines))
	cleanL2 := make([]budgetLineInput, 0, len(lines))

	seenL1 := map[uint]bool{}
	seenL2 := map[uint]bool{}

	l1Amount := map[uint]float64{}
	l2SumByL1 := map[uint]float64{}

	for _, l := range lines {
		amt := round2(l.Amount)
		if amt <= 0 {
			continue
		}
		if l.CategoryL1ID == nil || *l.CategoryL1ID == 0 {
			return fmt.Errorf("category_l1_id required")
		}
		l.Amount = amt
		l.Note = strings.TrimSpace(l.Note)

		if l.CategoryL2ID == nil || *l.CategoryL2ID == 0 {
			if seenL1[*l.CategoryL1ID] {
				return fmt.Errorf("duplicate category")
			}
			seenL1[*l.CategoryL1ID] = true
			l1Amount[*l.CategoryL1ID] = amt
			l.CategoryL2ID = nil
			cleanL1 = append(cleanL1, l)
			continue
		}

		if seenL2[*l.CategoryL2ID] {
			return fmt.Errorf("duplicate category")
		}
		seenL2[*l.CategoryL2ID] = true
		cleanL2 = append(cleanL2, l)
	}

	allocated := 0.0
	for _, l := range cleanL1 {
		allocated += l.Amount
	}
	if allocated > round2(totalAmount)+0.0001 {
		return fmt.Errorf("lines sum exceeds total")
	}

	if len(cleanL2) > 0 {
		ids := make([]uint, 0, len(cleanL2))
		for _, l := range cleanL2 {
			ids = append(ids, *l.CategoryL2ID)
		}
		type l2Row struct {
			ID       uint
			ParentID uint
		}
		var rows []l2Row
		if err := db.DB.Model(&model.CategoryL2{}).Select("id, parent_id").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return err
		}
		parentByL2 := map[uint]uint{}
		for _, r := range rows {
			parentByL2[r.ID] = r.ParentID
		}

		for _, l := range cleanL2 {
			l1 := *l.CategoryL1ID
			l2 := *l.CategoryL2ID
			parentID, ok := parentByL2[l2]
			if !ok {
				return fmt.Errorf("invalid category_l2_id")
			}
			if parentID != l1 {
				return fmt.Errorf("category_l2_id not match category_l1_id")
			}
			parentAmt, ok := l1Amount[l1]
			if !ok {
				return fmt.Errorf("category_l1_id required for category_l2 split")
			}
			l2SumByL1[l1] = round2(l2SumByL1[l1] + l.Amount)
			if l2SumByL1[l1] > round2(parentAmt)+0.0001 {
				return fmt.Errorf("l2 lines sum exceeds l1 budget")
			}
		}
	}

	if err := db.DB.Where("budget_id = ?", budgetID).Delete(&model.BudgetLine{}).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, l := range cleanL1 {
		bl := model.BudgetLine{
			BudgetID:     budgetID,
			CategoryL1ID: l.CategoryL1ID,
			CategoryL2ID: nil,
			Amount:       l.Amount,
			Note:         l.Note,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.DB.Create(&bl).Error; err != nil {
			return err
		}
	}
	for _, l := range cleanL2 {
		bl := model.BudgetLine{
			BudgetID:     budgetID,
			CategoryL1ID: l.CategoryL1ID,
			CategoryL2ID: l.CategoryL2ID,
			Amount:       l.Amount,
			Note:         l.Note,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.DB.Create(&bl).Error; err != nil {
			return err
		}
	}

	return nil
}

func activateBudget(budgetID string) error {
	var b model.Budget
	if err := db.DB.Where("id = ?", budgetID).First(&b).Error; err != nil {
		return fmt.Errorf("Budget not found")
	}
	if b.Status == "discarded" {
		return fmt.Errorf("Budget discarded")
	}

	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Budget{}).
			Where("month = ? AND status = ?", b.Month, "active").
			Where("id <> ?", b.ID).
			Updates(map[string]any{"status": "archived", "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Budget{}).
			Where("id = ?", b.ID).
			Updates(map[string]any{"status": "active", "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return nil
	})
}

func buildBudgetUsageDTO(budgetID string) (budgetUsageDTO, error) {
	var b model.Budget
	if err := db.DB.Where("id = ?", budgetID).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return budgetUsageDTO{}, gorm.ErrRecordNotFound
		}
		return budgetUsageDTO{}, err
	}

	thresholds := []budgetThresholdDTO{}
	_ = json.Unmarshal([]byte(b.Thresholds), &thresholds)

	usedTotal, err := computeMonthSpend(b.Month)
	if err != nil {
		return budgetUsageDTO{}, err
	}

	start, end, err := monthRange(b.Month)
	if err != nil {
		return budgetUsageDTO{}, err
	}

	type usedRow struct {
		CategoryID uint    `json:"category_id"`
		Amount     float64 `json:"amount"`
	}
	var rows []usedRow
	err = db.DB.Model(&model.Transaction{}).
		Select("category_l1_id as category_id, SUM(amount) as amount").
		Where("transaction_time BETWEEN ? AND ?", start, end).
		Group("category_l1_id").
		Scan(&rows).Error
	if err != nil {
		return budgetUsageDTO{}, err
	}
	usedByL1 := map[uint]float64{}
	for _, r := range rows {
		usedByL1[r.CategoryID] = round2(r.Amount)
	}

	var rowsL2 []usedRow
	err = db.DB.Model(&model.Transaction{}).
		Select("category_l2_id as category_id, SUM(amount) as amount").
		Where("transaction_time BETWEEN ? AND ?", start, end).
		Group("category_l2_id").
		Scan(&rowsL2).Error
	if err != nil {
		return budgetUsageDTO{}, err
	}
	usedByL2 := map[uint]float64{}
	for _, r := range rowsL2 {
		usedByL2[r.CategoryID] = round2(r.Amount)
	}

	var lines []model.BudgetLine
	if err := db.DB.Where("budget_id = ?", budgetID).Preload("CategoryL1").Preload("CategoryL2").Order("id asc").Find(&lines).Error; err != nil {
		return budgetUsageDTO{}, err
	}

	lineDTOs := make([]budgetLineUsageDTO, 0, len(lines)+8)

	allocatedBudget := 0.0
	allocatedUsed := 0.0
	l1Budget := map[uint]float64{}
	l1CatByID := map[uint]model.CategoryL1{}
	l2BudgetByL1 := map[uint]map[uint]float64{}
	for _, l := range lines {
		if l.CategoryL1ID == nil {
			continue
		}
		l1 := *l.CategoryL1ID
		amt := round2(l.Amount)
		if l.CategoryL2ID == nil {
			allocatedBudget += amt
			l1Budget[l1] = amt
			l1CatByID[l1] = l.CategoryL1
			continue
		}
		l2 := *l.CategoryL2ID
		if _, ok := l2BudgetByL1[l1]; !ok {
			l2BudgetByL1[l1] = map[uint]float64{}
		}
		l2BudgetByL1[l1][l2] = amt
	}

	for _, l := range lines {
		if l.CategoryL1ID == nil {
			continue
		}
		amt := round2(l.Amount)
		l1 := *l.CategoryL1ID
		if l.CategoryL2ID == nil {
			used := usedByL1[l1]
			allocatedUsed += used
			usageRatio := 0.0
			if amt > 0 {
				usageRatio = clamp01(used / amt)
			}
			c1 := l.CategoryL1
			lineDTOs = append(lineDTOs, budgetLineUsageDTO{
				ID:           l.ID,
				Scope:        "l1",
				CategoryL1ID: l.CategoryL1ID,
				CategoryL2ID: nil,
				CategoryL1:   &c1,
				CategoryL2:   nil,
				Amount:       amt,
				UsedAmount:   round2(used),
				Remaining:    round2(amt - used),
				UsageRatio:   usageRatio,
				Note:         l.Note,
			})
			continue
		}
		l2 := *l.CategoryL2ID
		used := usedByL2[l2]
		usageRatio := 0.0
		if amt > 0 {
			usageRatio = clamp01(used / amt)
		}
		c1 := l.CategoryL1
		c2 := l.CategoryL2
		lineDTOs = append(lineDTOs, budgetLineUsageDTO{
			ID:           l.ID,
			Scope:        "l2",
			CategoryL1ID: l.CategoryL1ID,
			CategoryL2ID: l.CategoryL2ID,
			CategoryL1:   &c1,
			CategoryL2:   &c2,
			Amount:       amt,
			UsedAmount:   round2(used),
			Remaining:    round2(amt - used),
			UsageRatio:   usageRatio,
			Note:         l.Note,
		})
	}

	for l1, amt := range l1Budget {
		l2BudgetSum := 0.0
		usedL2Sum := 0.0
		if m, ok := l2BudgetByL1[l1]; ok {
			for l2, a := range m {
				l2BudgetSum += a
				usedL2Sum += usedByL2[l2]
			}
		}
		unsplitAmt := round2(amt - l2BudgetSum)
		if unsplitAmt <= 0 {
			continue
		}
		unsplitUsed := round2(usedByL1[l1] - usedL2Sum)
		if unsplitUsed < 0 {
			unsplitUsed = 0
		}
		usageRatio := 0.0
		if unsplitAmt > 0 {
			usageRatio = clamp01(unsplitUsed / unsplitAmt)
		}
		l1id := l1
		var cat1 *model.CategoryL1
		if c1, ok := l1CatByID[l1]; ok {
			cc := c1
			cat1 = &cc
		}
		lineDTOs = append(lineDTOs, budgetLineUsageDTO{
			ID:           0,
			Scope:        "l1_unsplit",
			CategoryL1ID: &l1id,
			CategoryL2ID: nil,
			CategoryL1:   cat1,
			CategoryL2:   nil,
			Amount:       unsplitAmt,
			UsedAmount:   unsplitUsed,
			Remaining:    round2(unsplitAmt - unsplitUsed),
			UsageRatio:   usageRatio,
			Note:         "未分配预算",
		})
	}

	globalUnallocated := round2(b.TotalAmount - allocatedBudget)
	if globalUnallocated > 0 {
		usedUnallocated := round2(usedTotal - allocatedUsed)
		if usedUnallocated < 0 {
			usedUnallocated = 0
		}
		usageRatio := 0.0
		if globalUnallocated > 0 {
			usageRatio = clamp01(usedUnallocated / globalUnallocated)
		}
		lineDTOs = append(lineDTOs, budgetLineUsageDTO{
			ID:           0,
			Scope:        "global_unsplit",
			CategoryL1ID: nil,
			CategoryL2ID: nil,
			CategoryL1:   nil,
			CategoryL2:   nil,
			Amount:       globalUnallocated,
			UsedAmount:   usedUnallocated,
			Remaining:    round2(globalUnallocated - usedUnallocated),
			UsageRatio:   usageRatio,
			Note:         "未分配预算",
		})
	}

	out := budgetUsageDTO{
		BudgetID:    b.ID,
		Month:       b.Month,
		Status:      b.Status,
		TotalAmount: round2(b.TotalAmount),
		UsedAmount:  round2(usedTotal),
		Remaining:   round2(b.TotalAmount - usedTotal),
		Thresholds:  thresholds,
		CreatedAt:   b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   b.UpdatedAt.Format(time.RFC3339),
	}
	if b.DiscardedAt != nil {
		s := b.DiscardedAt.Format(time.RFC3339)
		out.DiscardedAt = &s
	}
	if b.TotalAmount > 0 {
		out.UsageRatio = clamp01(usedTotal / b.TotalAmount)
	} else {
		out.UsageRatio = 0
	}
	out.Waterline = evalWaterline(out.UsageRatio, thresholds)
	out.Lines = lineDTOs
	return out, nil
}
