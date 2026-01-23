package controller

import (
	"context"
	"encoding/json"
	"fmt"
	ai "ledger/api/ai"
	"ledger/api/db"
	dbModel "ledger/api/model"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ConsumptionAnalysisQuery struct {
	StartDate string `form:"start_date" binding:"required"`
	EndDate   string `form:"end_date" binding:"required"`
	TrendType string `form:"trend_type"`
	ProjectID string `form:"project_id"`
	Compare   string `form:"compare"`
}

type AnalysisRangeDTO struct {
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	CompareStartDate string `json:"compare_start_date,omitempty"`
	CompareEndDate   string `json:"compare_end_date,omitempty"`
}

type AnalysisKPIsDTO struct {
	TotalAmount float64  `json:"total_amount"`
	AvgPerDay   float64  `json:"avg_per_day"`
	DeltaAmount *float64 `json:"delta_amount,omitempty"`
	DeltaRatio  *float64 `json:"delta_ratio,omitempty"`
}

type NameAmountDTO struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

type NameAmountRatioDTO struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Ratio  float64 `json:"ratio"`
}

type EvidenceDTO struct {
	TopCategoriesL1 []NameAmountRatioDTO `json:"top_categories_l1"`
	TopMerchants    []NameAmountDTO      `json:"top_merchants"`
}

type TrendPointDTO struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

type AIAnalysisSplitDTO struct {
	Amount      float64         `json:"amount"`
	Ratio       float64         `json:"ratio"`
	BreakdownL1 []NameAmountDTO `json:"breakdown_l1"`
}

type AIBudgetSuggestionDTO struct {
	NextPeriodTotal *struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"next_period_total,omitempty"`
	NextPeriodRigid *struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"next_period_rigid,omitempty"`
	NextPeriodNonRigid *struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"next_period_non_rigid,omitempty"`
	ProjectBudget *struct {
		Enabled          bool     `json:"enabled"`
		Limit            float64  `json:"limit"`
		Status           string   `json:"status,omitempty"`
		RecommendedLimit *float64 `json:"recommended_limit,omitempty"`
	} `json:"project_budget,omitempty"`
}

type AIAnalysisSummaryDTO struct {
	Bullets []string `json:"bullets"`
	Caveats []string `json:"caveats,omitempty"`
}

type AIChangeInsightsDTO struct {
	TopIncrease *struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	} `json:"top_increase,omitempty"`
	TopDecrease *struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	} `json:"top_decrease,omitempty"`
}

type AIConsumptionAnalysisDTO struct {
	Rigid            AIAnalysisSplitDTO    `json:"rigid"`
	NonRigid         AIAnalysisSplitDTO    `json:"non_rigid"`
	ChangeInsights   AIChangeInsightsDTO   `json:"change_insights"`
	BudgetSuggestion AIBudgetSuggestionDTO `json:"budget_suggestion"`
	AISummary        AIAnalysisSummaryDTO  `json:"ai_summary"`
}

type ConsumptionAnalysisResponseDTO struct {
	Range            AnalysisRangeDTO      `json:"range"`
	KPIs             AnalysisKPIsDTO       `json:"kpis"`
	Trend            []TrendPointDTO       `json:"trend"`
	Evidence         EvidenceDTO           `json:"evidence"`
	Rigid            AIAnalysisSplitDTO    `json:"rigid"`
	NonRigid         AIAnalysisSplitDTO    `json:"non_rigid"`
	ChangeInsights   AIChangeInsightsDTO   `json:"change_insights"`
	BudgetSuggestion AIBudgetSuggestionDTO `json:"budget_suggestion"`
	AISummary        AIAnalysisSummaryDTO  `json:"ai_summary"`
}

func GetConsumptionAnalysis(c *gin.Context) {
	var req ConsumptionAnalysisQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trendType := strings.TrimSpace(req.TrendType)
	if trendType != "month" {
		trendType = "day"
	}
	compare := strings.TrimSpace(req.Compare)
	if compare == "" {
		compare = "previous"
	}
	if compare != "previous" && compare != "none" {
		compare = "previous"
	}

	startT, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date"})
		return
	}
	endT, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date"})
		return
	}
	if startT.After(endT) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date after end_date"})
		return
	}

	var project *dbModel.Project
	if strings.TrimSpace(req.ProjectID) != "" {
		var p dbModel.Project
		if err := db.DB.First(&p, req.ProjectID).Error; err == nil {
			project = &p
		}
	}

	totalAmount, err := sumAmountInRange(req.StartDate, req.EndDate, req.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	days := int(endT.Sub(startT).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	avgPerDay := round2(totalAmount / float64(days))

	resp := ConsumptionAnalysisResponseDTO{}
	resp.Range.StartDate = req.StartDate
	resp.Range.EndDate = req.EndDate
	resp.KPIs.TotalAmount = round2(totalAmount)
	resp.KPIs.AvgPerDay = avgPerDay

	var compareTotal *float64
	if compare == "previous" {
		compareEnd := startT.AddDate(0, 0, -1)
		compareStart := compareEnd.AddDate(0, 0, -(days - 1))
		cs := compareStart.Format("2006-01-02")
		ce := compareEnd.Format("2006-01-02")
		resp.Range.CompareStartDate = cs
		resp.Range.CompareEndDate = ce
		ct, err := sumAmountInRange(cs, ce, req.ProjectID)
		if err == nil {
			ct = round2(ct)
			compareTotal = &ct
			delta := round2(totalAmount - ct)
			resp.KPIs.DeltaAmount = &delta
			if ct > 0 {
				r := round4(delta / ct)
				resp.KPIs.DeltaRatio = &r
			} else {
				r := 0.0
				resp.KPIs.DeltaRatio = &r
			}
		}
	}

	trend, err := getTrendSeries(req.StartDate, req.EndDate, trendType, req.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.Trend = trend

	topCats, err := getTopCategoriesL1(req.StartDate, req.EndDate, req.ProjectID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.Evidence.TopCategoriesL1 = topCats

	topMerchants, err := getTopMerchants(req.StartDate, req.EndDate, req.ProjectID, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.Evidence.TopMerchants = topMerchants

	aiOut, err := callConsumptionAnalysisAI(consumptionAnalysisAIInput{
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		CompareTotal:  compareTotal,
		TotalAmount:   round2(totalAmount),
		AvgPerDay:     avgPerDay,
		TrendType:     trendType,
		Trend:         trend,
		TopCategories: topCats,
		TopMerchants:  topMerchants,
		Project:       project,
		Compare:       compare,
		CompareStart:  resp.Range.CompareStartDate,
		CompareEnd:    resp.Range.CompareEndDate,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp.Rigid = aiOut.Rigid
	resp.NonRigid = aiOut.NonRigid
	resp.ChangeInsights = aiOut.ChangeInsights
	resp.BudgetSuggestion = aiOut.BudgetSuggestion
	resp.AISummary = aiOut.AISummary
	log.Printf("AI ConsumptionAnalysis Output: %+v \n", aiOut)

	c.JSON(http.StatusOK, resp)
}

func sumAmountInRange(startDate, endDate, projectID string) (float64, error) {
	query := db.DB.Table("transactions").Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	var total float64
	if err := query.Select("COALESCE(SUM(transactions.amount), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func getTrendSeries(startDate, endDate, trendType, projectID string) ([]TrendPointDTO, error) {
	dateFormat := "%Y-%m-%d"
	layout := "2006-01-02"
	if trendType == "month" {
		dateFormat = "%Y-%m"
		layout = "2006-01"
		if len(startDate) > 7 {
			startDate = startDate[:7]
		}
		if len(endDate) > 7 {
			endDate = endDate[:7]
		}
	}

	type rawRow struct {
		Date   string
		Amount float64
	}
	var rows []rawRow

	query := db.DB.Table("transactions").
		Select("strftime(?, transactions.transaction_time) as date, SUM(transactions.amount) as amount", dateFormat).
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	err := query.Group("date").Order("date").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	dataMap := map[string]float64{}
	for _, r := range rows {
		dataMap[r.Date] = round2(r.Amount)
	}

	st, _ := time.ParseInLocation(layout, startDate, time.Local)
	ed, _ := time.ParseInLocation(layout, endDate, time.Local)

	var out []TrendPointDTO
	cur := st
	for !cur.After(ed) {
		k := cur.Format(layout)
		out = append(out, TrendPointDTO{Date: k, Amount: dataMap[k]})
		if trendType == "month" {
			cur = cur.AddDate(0, 1, 0)
		} else {
			cur = cur.AddDate(0, 0, 1)
		}
	}
	return out, nil
}

func getTopCategoriesL1(startDate, endDate, projectID string, limit int) ([]NameAmountRatioDTO, error) {
	type row struct {
		Name   string
		Amount float64
	}
	var rows []row
	query := db.DB.Table("transactions").
		Select("category_l1.name as name, SUM(transactions.amount) as amount").
		Joins("left join category_l1 on transactions.category_l1_id = category_l1.id").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	if err := query.Group("category_l1.name").Order("amount desc").Scan(&rows).Error; err != nil {
		return nil, err
	}

	total := 0.0
	for _, r := range rows {
		total += r.Amount
	}
	if total <= 0 {
		return []NameAmountRatioDTO{}, nil
	}

	if limit <= 0 {
		limit = 10
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}

	out := make([]NameAmountRatioDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, NameAmountRatioDTO{Name: r.Name, Amount: round2(r.Amount), Ratio: round4(r.Amount / total)})
	}
	return out, nil
}

func getTopMerchants(startDate, endDate, projectID string, limit int) ([]NameAmountDTO, error) {
	type row struct {
		Name   string
		Amount float64
	}
	var rows []row
	query := db.DB.Table("transactions").
		Select("COALESCE(NULLIF(TRIM(transactions.merchant), ''), '未知') as name, SUM(transactions.amount) as amount").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	if err := query.Group("name").Order("amount desc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]NameAmountDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, NameAmountDTO{Name: r.Name, Amount: round2(r.Amount)})
	}
	return out, nil
}

type consumptionAnalysisAIInput struct {
	StartDate     string
	EndDate       string
	Compare       string
	CompareStart  string
	CompareEnd    string
	TotalAmount   float64
	AvgPerDay     float64
	CompareTotal  *float64
	TrendType     string
	Trend         []TrendPointDTO
	TopCategories []NameAmountRatioDTO
	TopMerchants  []NameAmountDTO
	Project       *dbModel.Project
}

func callConsumptionAnalysisAI(input consumptionAnalysisAIInput) (AIConsumptionAnalysisDTO, error) {
	conf, err := getAIConfig()
	if err != nil {
		log.Printf("AI consumption analysis failed: config error: %v", err)
		return AIConsumptionAnalysisDTO{}, err
	}

	log.Printf("AI consumption analysis request: model=%s, total_amount=%.2f, start=%s, end=%s",
		conf.Model, input.TotalAmount, input.StartDate, input.EndDate)

	client := ai.NewClient(conf.BaseURL, conf.APIKey)

	payload := map[string]any{
		"range": map[string]any{
			"start_date":         input.StartDate,
			"end_date":           input.EndDate,
			"compare":            input.Compare,
			"compare_start_date": input.CompareStart,
			"compare_end_date":   input.CompareEnd,
		},
		"kpis": map[string]any{
			"total_amount":         input.TotalAmount,
			"avg_per_day":          input.AvgPerDay,
			"compare_total_amount": input.CompareTotal,
		},
		"trend": map[string]any{
			"trend_type": input.TrendType,
			"series":     input.Trend,
		},
		"evidence": map[string]any{
			"top_categories_l1": input.TopCategories,
			"top_merchants":     input.TopMerchants,
		},
		"project": func() any {
			if input.Project == nil {
				return nil
			}
			return map[string]any{
				"id":             input.Project.ID,
				"name":           input.Project.Name,
				"budget_enabled": input.Project.BudgetEnabled,
				"budget_limit":   input.Project.BudgetLimit,
			}
		}(),
	}
	bs, _ := json.Marshal(payload)

	prompt := fmt.Sprintf(`
You are a financial analysis assistant.

You will be given ONLY raw aggregated spending data (no rule-based rigidity labels are provided).
You must determine which spending is "rigid" (necessary) and "non_rigid" (adjustable) by reasoning about the categories/merchants/trends.

Requirements:
- Use the provided numbers. Do NOT invent categories or amounts.
- You may choose to classify each L1 category into rigid/non_rigid or split partially, but you must ensure:
  rigid.amount + non_rigid.amount == kpis.total_amount (within 0.01 tolerance).
- Provide an interpretable summary and budget suggestions.
- Output ONLY valid JSON, no markdown.

RawData(JSON):
%s

Return JSON schema:
{
  "rigid": {"amount": number, "ratio": number, "breakdown_l1": [{"name": string, "amount": number}]},
  "non_rigid": {"amount": number, "ratio": number, "breakdown_l1": [{"name": string, "amount": number}]},
  "change_insights": {"top_increase": {"name": string, "amount": number}?, "top_decrease": {"name": string, "amount": number}?},
  "budget_suggestion": {
    "next_period_total": {"min": number, "max": number}?,
    "next_period_rigid": {"min": number, "max": number}?,
    "next_period_non_rigid": {"min": number, "max": number}?,
    "project_budget": {"enabled": boolean, "limit": number, "status": "ok"|"exceed"?, "recommended_limit": number?}?
  },
  "ai_summary": {"bullets": [string], "caveats": [string]?}
}
`, string(bs))

	start := time.Now()
	content, err := client.CreateChatCompletion(context.Background(), ai.ChatCompletionRequest{
		Model: conf.Model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		log.Printf("AI consumption analysis failed after %v: %v", time.Since(start), err)
		return AIConsumptionAnalysisDTO{}, fmt.Errorf("AI Service Error: %v", err)
	}

	log.Printf("AI consumption analysis success: latency=%vms", time.Since(start).Milliseconds())
	content = stripCodeFence(content)

	var out AIConsumptionAnalysisDTO
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		log.Printf("AI consumption analysis parse failed: %v, content=%s", err, content)
		return AIConsumptionAnalysisDTO{}, fmt.Errorf("Failed to parse AI response, content: [%s]", content)
	}

	if out.Rigid.BreakdownL1 == nil {
		out.Rigid.BreakdownL1 = []NameAmountDTO{}
	}
	if out.NonRigid.BreakdownL1 == nil {
		out.NonRigid.BreakdownL1 = []NameAmountDTO{}
	}
	if out.AISummary.Bullets == nil {
		out.AISummary.Bullets = []string{}
	}
	if out.AISummary.Caveats == nil {
		out.AISummary.Caveats = []string{}
	}

	out.Rigid.Amount = round2(out.Rigid.Amount)
	out.NonRigid.Amount = round2(out.NonRigid.Amount)
	out.Rigid.Ratio = clamp01(out.Rigid.Ratio)
	out.NonRigid.Ratio = clamp01(out.NonRigid.Ratio)

	for i := range out.Rigid.BreakdownL1 {
		out.Rigid.BreakdownL1[i].Amount = round2(out.Rigid.BreakdownL1[i].Amount)
	}
	for i := range out.NonRigid.BreakdownL1 {
		out.NonRigid.BreakdownL1[i].Amount = round2(out.NonRigid.BreakdownL1[i].Amount)
	}
	sort.Slice(out.Rigid.BreakdownL1, func(i, j int) bool { return out.Rigid.BreakdownL1[i].Amount > out.Rigid.BreakdownL1[j].Amount })
	sort.Slice(out.NonRigid.BreakdownL1, func(i, j int) bool { return out.NonRigid.BreakdownL1[i].Amount > out.NonRigid.BreakdownL1[j].Amount })

	return out, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
