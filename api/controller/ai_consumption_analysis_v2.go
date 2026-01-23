package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ai "ledger/api/ai"
	"ledger/api/db"
	"ledger/api/model"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ConsumptionAnalysisCreateRequest struct {
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	TrendType       string `json:"trend_type"`
	ProjectID       string `json:"project_id"`
	Compare         string `json:"compare"`
	ForceRegenerate bool   `json:"force_regenerate"`
	UserRequirement string `json:"user_requirement"` // 用户自然语言输入
}

type ConsumptionAnalysisLabelDTO struct {
	TransactionID string   `json:"transaction_id"`
	Label         string   `json:"label"`      // rigid / non_rigid
	Assessment    string   `json:"assessment"` // reasonable / suspicious / wasteful
	Reason        string   `json:"reason"`
	Confidence    *float64 `json:"confidence,omitempty"`
}

type ConsumptionAnalysisV2ResponseDTO struct {
	AnalysisID                 string                         `json:"analysis_id"`
	CreatedAt                  string                         `json:"created_at"`
	Language                   string                         `json:"language"`
	Range                      AnalysisRangeDTO               `json:"range"`
	KPIs                       AnalysisKPIsDTO                `json:"kpis"`
	Trend                      []TrendPointDTO                `json:"trend"`
	Transactions               []txForAI                      `json:"transactions"`
	Evidence                   EvidenceDTO                    `json:"evidence"`
	Rigid                      AIAnalysisSplitDTO             `json:"rigid"`
	NonRigid                   AIAnalysisSplitDTO             `json:"non_rigid"`
	TransactionLabels          []ConsumptionAnalysisLabelDTO  `json:"transaction_labels"`
	RemovableNonEssentialItems []RemovableNonEssentialItemDTO `json:"removable_non_essential_items"`
	ChangeInsights             AIChangeInsightsDTO            `json:"change_insights"`
	BudgetSuggestion           AIBudgetSuggestionDTO          `json:"budget_suggestion"`
	AISummary                  AIAnalysisSummaryDTO           `json:"ai_summary"`
}

type RemovableNonEssentialItemDTO struct {
	Title            string   `json:"title"`
	CategoryL1       *string  `json:"category_l1,omitempty"`
	Merchant         *string  `json:"merchant,omitempty"`
	Amount           *float64 `json:"amount,omitempty"`
	FrequencyHint    *string  `json:"frequency_hint,omitempty"`
	Reason           string   `json:"reason"`
	ActionSuggestion string   `json:"action_suggestion"`
	EstimatedSaving  *struct {
		Min *float64 `json:"min,omitempty"`
		Max *float64 `json:"max,omitempty"`
	} `json:"estimated_saving,omitempty"`
	Confidence *string `json:"confidence,omitempty"`
}

type ConsumptionAnalysisHistoryItemDTO struct {
	AnalysisID       string `json:"analysis_id"`
	CreatedAt        string `json:"created_at"`
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg,omitempty"`
	DurationMs       *int64 `json:"duration_ms,omitempty"`
	TransactionCount int    `json:"transaction_count"`
	Range            struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	} `json:"range"`
	ProjectID *uint `json:"project_id,omitempty"`
	KPIs      struct {
		TotalAmount float64 `json:"total_amount"`
		AvgPerDay   float64 `json:"avg_per_day"`
	} `json:"kpis"`
}

type ConsumptionAnalysisHistoryResponseDTO struct {
	Items []ConsumptionAnalysisHistoryItemDTO `json:"items"`
	Total int64                               `json:"total"`
}

type ConsumptionAnalysisStatusDTO struct {
	AnalysisID       string  `json:"analysis_id"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	StartedAt        *string `json:"started_at,omitempty"`
	FinishedAt       *string `json:"finished_at,omitempty"`
	DurationMs       *int64  `json:"duration_ms,omitempty"`
	ErrorMsg         string  `json:"error_msg,omitempty"`
	PromptLength     int     `json:"prompt_length"`
	TransactionCount int     `json:"transaction_count"`
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func buildConsumptionAnalysisStatus(run model.AnalysisRun) ConsumptionAnalysisStatusDTO {
	status := run.Status
	if status == "" {
		status = analysisStatusCompleted
	}
	return ConsumptionAnalysisStatusDTO{
		AnalysisID:       run.ID,
		Status:           status,
		CreatedAt:        run.CreatedAt.Format(time.RFC3339),
		StartedAt:        formatTimePtr(run.StartedAt),
		FinishedAt:       formatTimePtr(run.FinishedAt),
		DurationMs:       run.DurationMs,
		ErrorMsg:         strings.TrimSpace(run.ErrorMsg),
		PromptLength:     run.PromptLength,
		TransactionCount: run.TransactionCount,
	}
}

type txForAI struct {
	TransactionID string  `json:"transaction_id"`
	Time          string  `json:"time"`
	Merchant      string  `json:"merchant"`
	Amount        float64 `json:"amount"`
	CategoryL1    string  `json:"category_l1"`
	CategoryL2    string  `json:"category_l2"`
	Remark        string  `json:"remark"`
}

const (
	analysisStatusPending   = "pending"
	analysisStatusRunning   = "running"
	analysisStatusCompleted = "completed"
	analysisStatusFailed    = "failed"
)

func PostConsumptionAnalysisV2(c *gin.Context) {
	var body ConsumptionAnalysisCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	body.StartDate = strings.TrimSpace(body.StartDate)
	body.EndDate = strings.TrimSpace(body.EndDate)
	if body.StartDate == "" || body.EndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}

	trendType := strings.TrimSpace(body.TrendType)
	if trendType != "month" {
		trendType = "day"
	}
	compare := "none"

	userIDAny, _ := c.Get("userID")
	userID, _ := userIDAny.(uint)

	var projectIDPtr *uint
	if strings.TrimSpace(body.ProjectID) != "" {
		if n, err := strconv.ParseUint(body.ProjectID, 10, 64); err == nil {
			u := uint(n)
			projectIDPtr = &u
		}
	}

	requestKey := struct {
		UserID          uint   `json:"user_id"`
		ProjectID       string `json:"project_id"`
		StartDate       string `json:"start_date"`
		EndDate         string `json:"end_date"`
		Compare         string `json:"compare"`
		TrendType       string `json:"trend_type"`
		Language        string `json:"language"`
		UserRequirement string `json:"user_requirement"`
	}{
		UserID:          userID,
		ProjectID:       strings.TrimSpace(body.ProjectID),
		StartDate:       body.StartDate,
		EndDate:         body.EndDate,
		Compare:         compare,
		TrendType:       trendType,
		Language:        "zh-CN",
		UserRequirement: strings.TrimSpace(body.UserRequirement),
	}
	requestKeyBytes, _ := json.Marshal(requestKey)
	h := sha256.Sum256(requestKeyBytes)
	requestHash := hex.EncodeToString(h[:])

	if !body.ForceRegenerate {
		var existing model.AnalysisRun
		err := db.DB.Where("request_hash = ?", requestHash).Order("created_at desc").First(&existing).Error
		if err == nil {
			if existing.Status == "" || existing.Status == analysisStatusCompleted {
				result := map[string]any{}
				if e := json.Unmarshal([]byte(existing.ResultJSON), &result); e == nil {
					result["analysis_id"] = existing.ID
					result["created_at"] = existing.CreatedAt.Format(time.RFC3339)
					result["language"] = existing.Language
					result["status"] = analysisStatusCompleted
					log.Printf("ai analysis cache hit analysis_id=%s", existing.ID)
					c.JSON(http.StatusOK, result)
					return
				}
			}
			c.JSON(http.StatusAccepted, buildConsumptionAnalysisStatus(existing))
			return
		}
	}

	analysisID := uuid.NewString()
	now := time.Now()
	requestPayload := map[string]any{
		"start_date":       body.StartDate,
		"end_date":         body.EndDate,
		"compare":          compare,
		"trend_type":       trendType,
		"project_id":       strings.TrimSpace(body.ProjectID),
		"language":         "zh-CN",
		"user_requirement": strings.TrimSpace(body.UserRequirement),
	}
	requestBytes, _ := json.Marshal(requestPayload)

	run := model.AnalysisRun{
		ID:          analysisID,
		UserID:      userID,
		ProjectID:   projectIDPtr,
		StartDate:   body.StartDate,
		EndDate:     body.EndDate,
		Compare:     compare,
		TrendType:   trendType,
		Language:    "zh-CN",
		RequestHash: requestHash,
		RequestJSON: string(requestBytes),
		Status:      analysisStatusPending,
		CreatedAt:   now,
	}
	if err := db.DB.Create(&run).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("ai analysis queued user_id=%d analysis_id=%s start=%s end=%s compare=%s trend=%s", userID, analysisID, body.StartDate, body.EndDate, compare, trendType)

	go runConsumptionAnalysisV2(analysisID)

	c.JSON(http.StatusAccepted, buildConsumptionAnalysisStatus(run))
}

func GetConsumptionAnalysisHistoryV2(c *gin.Context) {
	//userIDAny, _ := c.Get("userID")
	//userID, _ := userIDAny.(uint)

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	projectID := strings.TrimSpace(c.Query("project_id"))
	limit := parseIntDefault(c.Query("limit"), 20)
	offset := parseIntDefault(c.Query("offset"), 0)
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := db.DB.Model(&model.AnalysisRun{})
	if startDate != "" {
		query = query.Where("start_date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("end_date <= ?", endDate)
	}
	if projectID != "" {
		if n, err := strconv.ParseUint(projectID, 10, 64); err == nil {
			query = query.Where("project_id = ?", uint(n))
		}
	}

	var total int64
	query.Count(&total)

	var runs []model.AnalysisRun
	if err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&runs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]ConsumptionAnalysisHistoryItemDTO, 0, len(runs))
	for _, r := range runs {
		status := r.Status
		if status == "" {
			status = analysisStatusCompleted
		}
		var parsed struct {
			Range struct {
				StartDate string `json:"start_date"`
				EndDate   string `json:"end_date"`
			} `json:"range"`
			KPIs struct {
				TotalAmount float64 `json:"total_amount"`
				AvgPerDay   float64 `json:"avg_per_day"`
			} `json:"kpis"`
		}
		if strings.TrimSpace(r.ResultJSON) != "" {
			_ = json.Unmarshal([]byte(r.ResultJSON), &parsed)
		}

		item := ConsumptionAnalysisHistoryItemDTO{
			AnalysisID:       r.ID,
			CreatedAt:        r.CreatedAt.Format(time.RFC3339),
			Status:           status,
			ErrorMsg:         strings.TrimSpace(r.ErrorMsg),
			DurationMs:       r.DurationMs,
			TransactionCount: r.TransactionCount,
			ProjectID:        r.ProjectID,
		}
		item.Range.StartDate = firstNonEmpty(parsed.Range.StartDate, r.StartDate)
		item.Range.EndDate = firstNonEmpty(parsed.Range.EndDate, r.EndDate)
		item.KPIs.TotalAmount = round2(parsed.KPIs.TotalAmount)
		item.KPIs.AvgPerDay = round2(parsed.KPIs.AvgPerDay)
		items = append(items, item)
	}

	c.JSON(http.StatusOK, ConsumptionAnalysisHistoryResponseDTO{Items: items, Total: total})
}

func GetConsumptionAnalysisDetailV2(c *gin.Context) {
	id := c.Param("analysis_id")
	var run model.AnalysisRun
	if err := db.DB.Where("id = ?", id).First(&run).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	status := run.Status
	if status == "" {
		status = analysisStatusCompleted
	}
	if status != analysisStatusCompleted || strings.TrimSpace(run.ResultJSON) == "" {
		c.JSON(http.StatusOK, buildConsumptionAnalysisStatus(run))
		return
	}

	result := map[string]any{}
	if err := json.Unmarshal([]byte(run.ResultJSON), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse stored result"})
		return
	}
	result["analysis_id"] = run.ID
	result["created_at"] = run.CreatedAt.Format(time.RFC3339)
	result["language"] = run.Language
	result["status"] = status
	result["started_at"] = formatTimePtr(run.StartedAt)
	result["finished_at"] = formatTimePtr(run.FinishedAt)
	result["duration_ms"] = run.DurationMs
	result["prompt_length"] = run.PromptLength
	result["transaction_count"] = run.TransactionCount

	log.Printf("ai analysis detail analysis_id=%s", run.ID)

	c.JSON(http.StatusOK, result)
}

func runConsumptionAnalysisV2(analysisID string) {
	startAt := time.Now()
	var run model.AnalysisRun
	if err := db.DB.Where("id = ?", analysisID).First(&run).Error; err != nil {
		log.Printf("ai analysis run missing analysis_id=%s error=%v", analysisID, err)
		return
	}
	startedAt := time.Now()
	if err := db.DB.Model(&model.AnalysisRun{}).Where("id = ?", analysisID).Updates(map[string]any{
		"status":     analysisStatusRunning,
		"started_at": startedAt,
		"error_msg":  "",
	}).Error; err != nil {
		log.Printf("ai analysis run update failed analysis_id=%s error=%v", analysisID, err)
		return
	}

	var req struct {
		StartDate       string `json:"start_date"`
		EndDate         string `json:"end_date"`
		TrendType       string `json:"trend_type"`
		ProjectID       string `json:"project_id"`
		Language        string `json:"language"`
		UserRequirement string `json:"user_requirement"`
	}
	if err := json.Unmarshal([]byte(run.RequestJSON), &req); err != nil {
		finalizeConsumptionAnalysisFailure(analysisID, startAt, fmt.Sprintf("invalid request payload: %v", err))
		return
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}
	maxTx := 500
	listStart := time.Now()
	txs, err := listTransactionsForAI(req.StartDate, req.EndDate, req.ProjectID, maxTx)
	listMs := time.Since(listStart).Milliseconds()
	if err != nil {
		finalizeConsumptionAnalysisFailure(analysisID, startAt, err.Error())
		log.Printf("ai analysis data error analysis_id=%s ms=%d error=%v", analysisID, listMs, err)
		return
	}
	if len(txs) >= maxTx {
		finalizeConsumptionAnalysisFailure(analysisID, startAt, fmt.Sprintf("该时间范围内交易笔数过多（≥%d），请缩小范围", maxTx))
		log.Printf("ai analysis data too many analysis_id=%s ms=%d count=%d", analysisID, listMs, len(txs))
		return
	}
	if err := db.DB.Model(&model.AnalysisRun{}).Where("id = ?", analysisID).Update("transaction_count", len(txs)).Error; err != nil {
		log.Printf("ai analysis tx count update failed analysis_id=%s error=%v", analysisID, err)
	}

	buildStart := time.Now()
	resp, requestPayload, err := buildConsumptionAnalysisV2(analysisID, run.UserID, req.StartDate, req.EndDate, "none", req.TrendType, req.ProjectID, req.UserRequirement, txs)
	buildMs := time.Since(buildStart).Milliseconds()
	if err != nil {
		finalizeConsumptionAnalysisFailure(analysisID, startAt, err.Error())
		log.Printf("ai analysis build error analysis_id=%s ms=%d error=%v", analysisID, buildMs, err)
		return
	}

	resp.AnalysisID = run.ID
	resp.CreatedAt = run.CreatedAt.Format(time.RFC3339)
	resp.Language = req.Language

	resultBytes, _ := json.Marshal(resp)
	requestBytes, _ := json.Marshal(requestPayload)
	finishedAt := time.Now()
	duration := finishedAt.Sub(startAt).Milliseconds()

	if err := db.DB.Model(&model.AnalysisRun{}).Where("id = ?", analysisID).Updates(map[string]any{
		"status":       analysisStatusCompleted,
		"result_json":  string(resultBytes),
		"request_json": string(requestBytes),
		"finished_at":  finishedAt,
		"duration_ms":  duration,
		"error_msg":    "",
	}).Error; err != nil {
		log.Printf("ai analysis save error analysis_id=%s error=%v", analysisID, err)
		return
	}

	log.Printf("ai analysis completed analysis_id=%s tx=%d list_ms=%d build_ms=%d total_ms=%d", analysisID, len(txs), listMs, buildMs, duration)
}

func finalizeConsumptionAnalysisFailure(analysisID string, startedAt time.Time, errMsg string) {
	finishedAt := time.Now()
	duration := finishedAt.Sub(startedAt).Milliseconds()
	_ = db.DB.Model(&model.AnalysisRun{}).Where("id = ?", analysisID).Updates(map[string]any{
		"status":      analysisStatusFailed,
		"error_msg":   errMsg,
		"finished_at": finishedAt,
		"duration_ms": duration,
	}).Error
	log.Printf("ai analysis failed analysis_id=%s total_ms=%d error=%s", analysisID, duration, errMsg)
}

func buildConsumptionAnalysisV2(analysisID string, userID uint, startDate, endDate, compare, trendType, projectID, userRequirement string, txs []txForAI) (ConsumptionAnalysisV2ResponseDTO, map[string]any, error) {
	startT, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, fmt.Errorf("invalid start_date")
	}
	endT, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, fmt.Errorf("invalid end_date")
	}
	if startT.After(endT) {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, fmt.Errorf("start_date after end_date")
	}

	var project *model.Project
	if strings.TrimSpace(projectID) != "" {
		var p model.Project
		if err := db.DB.First(&p, projectID).Error; err == nil {
			project = &p
		}
	}

	totalAmount, err := sumAmountInRangeForUser(startDate, endDate, projectID)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, err
	}

	days := int(endT.Sub(startT).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	avgPerDay := round2(totalAmount / float64(days))

	resp := ConsumptionAnalysisV2ResponseDTO{}
	resp.Range.StartDate = startDate
	resp.Range.EndDate = endDate
	resp.KPIs.TotalAmount = round2(totalAmount)
	resp.KPIs.AvgPerDay = avgPerDay
	resp.Transactions = txs

	var compareTotal *float64
	if compare == "previous" {
		compareEnd := startT.AddDate(0, 0, -1)
		compareStart := compareEnd.AddDate(0, 0, -(days - 1))
		cs := compareStart.Format("2006-01-02")
		ce := compareEnd.Format("2006-01-02")
		resp.Range.CompareStartDate = cs
		resp.Range.CompareEndDate = ce
		ct, err := sumAmountInRangeForUser(cs, ce, projectID)
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

	trend, err := getTrendSeriesForUser(startDate, endDate, trendType, projectID)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, err
	}
	resp.Trend = trend

	topCats, err := getTopCategoriesL1ForUser(startDate, endDate, projectID, 10)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, err
	}
	resp.Evidence.TopCategoriesL1 = topCats

	topMerchants, err := getTopMerchantsForUser(startDate, endDate, projectID, 10)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, err
	}
	resp.Evidence.TopMerchants = topMerchants

	requestPayload := map[string]any{
		"language": "zh-CN",
		"range": map[string]any{
			"start_date":         startDate,
			"end_date":           endDate,
			"compare":            compare,
			"compare_start_date": resp.Range.CompareStartDate,
			"compare_end_date":   resp.Range.CompareEndDate,
		},
		"kpis": map[string]any{
			"total_amount":         round2(totalAmount),
			"avg_per_day":          avgPerDay,
			"compare_total_amount": compareTotal,
		},
		"trend": map[string]any{
			"trend_type": trendType,
			"series":     trend,
		},
		"evidence": map[string]any{
			"top_categories_l1": topCats,
			"top_merchants":     topMerchants,
		},
		"transactions":     txs,
		"user_requirement": userRequirement,
		"project": func() any {
			if project == nil {
				return nil
			}
			return map[string]any{
				"id":             project.ID,
				"name":           project.Name,
				"budget_enabled": project.BudgetEnabled,
				"budget_limit":   project.BudgetLimit,
			}
		}(),
	}

	aiOut, err := callConsumptionAnalysisAIV2(analysisID, requestPayload, txs)
	if err != nil {
		return ConsumptionAnalysisV2ResponseDTO{}, nil, err
	}

	resp.Rigid = aiOut.Rigid
	resp.NonRigid = aiOut.NonRigid
	resp.TransactionLabels = aiOut.TransactionLabels
	resp.RemovableNonEssentialItems = aiOut.RemovableNonEssentialItems
	resp.ChangeInsights = aiOut.ChangeInsights
	resp.BudgetSuggestion = aiOut.BudgetSuggestion
	resp.AISummary = aiOut.AISummary

	return resp, requestPayload, nil
}

type aiV2Out struct {
	Rigid                      AIAnalysisSplitDTO             `json:"rigid"`
	NonRigid                   AIAnalysisSplitDTO             `json:"non_rigid"`
	TransactionLabels          []ConsumptionAnalysisLabelDTO  `json:"transaction_labels"`
	RemovableNonEssentialItems []RemovableNonEssentialItemDTO `json:"removable_non_essential_items"`
	ChangeInsights             AIChangeInsightsDTO            `json:"change_insights"`
	BudgetSuggestion           AIBudgetSuggestionDTO          `json:"budget_suggestion"`
	AISummary                  AIAnalysisSummaryDTO           `json:"ai_summary"`
}

type aiV2LabelOut struct {
	TransactionLabels []ConsumptionAnalysisLabelDTO `json:"transaction_labels"`
}

type aiV2InsightsOut struct {
	RemovableNonEssentialItems []RemovableNonEssentialItemDTO `json:"removable_non_essential_items"`
	ChangeInsights             AIChangeInsightsDTO            `json:"change_insights"`
	BudgetSuggestion           AIBudgetSuggestionDTO          `json:"budget_suggestion"`
	AISummary                  AIAnalysisSummaryDTO           `json:"ai_summary"`
}

func callConsumptionAnalysisAIV2(analysisID string, requestPayload map[string]any, txs []txForAI) (aiV2Out, error) {
	conf, err := getAIConfig()
	if err != nil {
		return aiV2Out{}, err
	}

	client := ai.NewClient(conf.BaseURL, conf.APIKey, ai.WithTimeout(600*time.Second))

	chunks := chunkTransactions(txs, 10)
	labels := make([]ConsumptionAnalysisLabelDTO, 0, len(txs))
	totalPromptLen := 0
	if len(chunks) > 0 {
		maxConcurrency := 4
		if len(chunks) < maxConcurrency {
			maxConcurrency = len(chunks)
		}
		type chunkJob struct {
			Index int
			Chunk []txForAI
		}
		type chunkResult struct {
			Index     int
			Labels    []ConsumptionAnalysisLabelDTO
			PromptLen int
			Err       error
		}
		jobs := make(chan chunkJob, len(chunks))
		results := make(chan chunkResult, len(chunks))
		for w := 0; w < maxConcurrency; w++ {
			go func() {
				for job := range jobs {
					chunkLabels, promptLen, err := callConsumptionAnalysisLabelsSubtaskWithRetry(analysisID, client, conf.Model, job.Index+1, len(chunks), job.Chunk)
					results <- chunkResult{Index: job.Index, Labels: chunkLabels, PromptLen: promptLen, Err: err}
				}
			}()
		}
		for i, chunk := range chunks {
			jobs <- chunkJob{Index: i, Chunk: chunk}
		}
		close(jobs)

		byIndex := make([]chunkResult, len(chunks))
		var firstErr error
		for i := 0; i < len(chunks); i++ {
			res := <-results
			byIndex[res.Index] = res
			if res.Err != nil && firstErr == nil {
				firstErr = res.Err
			}
		}
		if firstErr != nil {
			return aiV2Out{}, firstErr
		}
		for _, res := range byIndex {
			totalPromptLen += res.PromptLen
			labels = append(labels, res.Labels...)
		}
	}
	if len(labels) != len(txs) {
		return aiV2Out{}, fmt.Errorf("AI 输出流水标注数量不完整")
	}

	rigid, nonRigid := computeSplitFromLabels(txs, labels)
	labelStats := buildLabelStats(txs, labels)

	insights, promptLen, err := callConsumptionAnalysisInsightsSubtask(analysisID, client, conf.Model, requestPayload, rigid, nonRigid, labelStats)
	if err != nil {
		return aiV2Out{}, err
	}
	totalPromptLen += promptLen
	_ = db.DB.Model(&model.AnalysisRun{}).Where("id = ?", analysisID).Update("prompt_length", totalPromptLen).Error

	return aiV2Out{
		Rigid:                      rigid,
		NonRigid:                   nonRigid,
		TransactionLabels:          labels,
		RemovableNonEssentialItems: insights.RemovableNonEssentialItems,
		ChangeInsights:             insights.ChangeInsights,
		BudgetSuggestion:           insights.BudgetSuggestion,
		AISummary:                  insights.AISummary,
	}, nil
}

type labelStatsDTO struct {
	TotalCount            int             `json:"total_count"`
	RigidCount            int             `json:"rigid_count"`
	NonRigidCount         int             `json:"non_rigid_count"`
	ReasonableCount       int             `json:"reasonable_count"`
	SuspiciousCount       int             `json:"suspicious_count"`
	WastefulCount         int             `json:"wasteful_count"`
	TopNonRigidCategories []NameAmountDTO `json:"top_non_rigid_categories"`
	TopWastefulCategories []NameAmountDTO `json:"top_wasteful_categories"`
	TopWastefulMerchants  []NameAmountDTO `json:"top_wasteful_merchants"`
}

func chunkTransactions(txs []txForAI, size int) [][]txForAI {
	if len(txs) == 0 {
		return [][]txForAI{}
	}
	if size <= 0 {
		size = len(txs)
	}
	chunks := make([][]txForAI, 0, (len(txs)+size-1)/size)
	for i := 0; i < len(txs); i += size {
		end := i + size
		if end > len(txs) {
			end = len(txs)
		}
		chunks = append(chunks, txs[i:end])
	}
	return chunks
}

func callConsumptionAnalysisLabelsSubtask(analysisID string, client *ai.Client, model string, index, total int, txs []txForAI) ([]ConsumptionAnalysisLabelDTO, int, error) {
	payload := map[string]any{
		"transactions": txs,
	}
	bs, _ := json.Marshal(payload)
	prompt := fmt.Sprintf(`你是一名专业的理财顾问，请只完成“逐笔合理性分析”。
输出必须是中文 JSON，且必须覆盖所有输入 transaction_id。
label 只能是 "rigid" 或 "non_rigid"。
assessment 只能是 "reasonable" 或 "suspicious" 或 "wasteful"。

输入数据（JSON）：
%s

返回 JSON schema（严格遵守）：
{
  "transaction_labels": [{"transaction_id": string, "label": "rigid"|"non_rigid", "assessment": "reasonable"|"suspicious"|"wasteful", "reason": string, "confidence": number?}]
}`, string(bs))

	promptLen := utf8.RuneCountInString(prompt)
	start := time.Now()
	log.Printf("ai v2 labels start analysis_id=%s chunk=%d/%d tx=%d prompt_len=%d payload_bytes=%d", analysisID, index, total, len(txs), promptLen, len(bs))
	content, err := client.CreateChatCompletion(context.Background(), ai.ChatCompletionRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, promptLen, fmt.Errorf("labels subtask error: %v", err)
	}
	log.Printf("ai v2 labels done analysis_id=%s chunk=%d/%d ms=%d", analysisID, index, total, time.Since(start).Milliseconds())
	content = stripCodeFence(content)

	var out aiV2LabelOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		snippet := content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, promptLen, fmt.Errorf("labels subtask parse error: %v (content_snippet=%s)", err, snippet)
	}
	if out.TransactionLabels == nil {
		out.TransactionLabels = []ConsumptionAnalysisLabelDTO{}
	}
	if len(out.TransactionLabels) != len(txs) {
		return nil, promptLen, fmt.Errorf("labels subtask count mismatch")
	}
	for i := range out.TransactionLabels {
		out.TransactionLabels[i].Label = strings.TrimSpace(out.TransactionLabels[i].Label)
		out.TransactionLabels[i].Assessment = strings.TrimSpace(out.TransactionLabels[i].Assessment)
		out.TransactionLabels[i].Reason = strings.TrimSpace(out.TransactionLabels[i].Reason)
	}
	return out.TransactionLabels, promptLen, nil
}

func callConsumptionAnalysisLabelsSubtaskWithRetry(analysisID string, client *ai.Client, model string, index, total int, txs []txForAI) ([]ConsumptionAnalysisLabelDTO, int, error) {
	maxRetries := 3
	sleep := 5 * time.Second
	var lastErr error
	var promptLen int
	for attempt := 0; attempt <= maxRetries; attempt++ {
		labels, pl, err := callConsumptionAnalysisLabelsSubtask(analysisID, client, model, index, total, txs)
		promptLen = pl
		if err == nil {
			return labels, promptLen, nil
		}
		lastErr = err
		if attempt == maxRetries {
			break
		}
		log.Printf("ai v2 labels retry analysis_id=%s chunk=%d/%d attempt=%d sleep=%ds error=%v", analysisID, index, total, attempt+1, int(sleep.Seconds()), err)
		time.Sleep(sleep)
		sleep *= 2
	}
	return nil, promptLen, lastErr
}

func callConsumptionAnalysisInsightsSubtask(analysisID string, client *ai.Client, model string, requestPayload map[string]any, rigid AIAnalysisSplitDTO, nonRigid AIAnalysisSplitDTO, stats labelStatsDTO) (aiV2InsightsOut, int, error) {
	insightPayload := map[string]any{
		"language":         requestPayload["language"],
		"range":            requestPayload["range"],
		"kpis":             requestPayload["kpis"],
		"trend":            requestPayload["trend"],
		"evidence":         requestPayload["evidence"],
		"project":          requestPayload["project"],
		"user_requirement": requestPayload["user_requirement"],
		"rigid":            rigid,
		"non_rigid":        nonRigid,
		"label_stats":      stats,
	}
	bs, _ := json.Marshal(insightPayload)
	prompt := fmt.Sprintf(`你是一名专业且贴心的理财顾问。
你将收到汇总后的消费数据、刚性/非刚性拆分以及标注统计。
请输出综合分析与建议（必须用中文输出 JSON）：
1. 分析消费模式，指出异常点。
2. 找出可剔除的非刚需项 (removable_non_essential_items)。
3. 结合历史数据和 **user_requirement** 生成下期预算建议。

输入数据（JSON）：
%s

返回 JSON schema（严格遵守）：
{
  "removable_non_essential_items": [{"title": string, "category_l1": string?, "merchant": string?, "amount": number?, "frequency_hint": string?, "reason": string, "action_suggestion": string, "estimated_saving": {"min": number?, "max": number?}?, "confidence": "low"|"medium"|"high"?}],
  "change_insights": {"top_increase": {"name": string, "amount": number}?, "top_decrease": {"name": string, "amount": number}?},
  "budget_suggestion": {
    "next_period_total": {"min": number, "max": number}?,
    "next_period_rigid": {"min": number, "max": number}?,
    "next_period_non_rigid": {"min": number, "max": number}?,
    "project_budget": {"enabled": boolean, "limit": number, "status": "ok"|"exceed"?, "recommended_limit": number?}?
  },
  "ai_summary": {"bullets": [string], "caveats": [string]?}
}`, string(bs))

	promptLen := utf8.RuneCountInString(prompt)
	start := time.Now()
	log.Printf("ai v2 insights start analysis_id=%s prompt_len=%d payload_bytes=%d", analysisID, promptLen, len(bs))
	content, err := client.CreateChatCompletion(context.Background(), ai.ChatCompletionRequest{
		Model: model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return aiV2InsightsOut{}, promptLen, fmt.Errorf("insights subtask error: %v", err)
	}
	log.Printf("ai v2 insights done analysis_id=%s ms=%d", analysisID, time.Since(start).Milliseconds())
	content = stripCodeFence(content)

	var out aiV2InsightsOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return aiV2InsightsOut{}, promptLen, fmt.Errorf("insights subtask parse error: %v", err)
	}
	if out.RemovableNonEssentialItems == nil {
		out.RemovableNonEssentialItems = []RemovableNonEssentialItemDTO{}
	}
	if out.AISummary.Bullets == nil {
		out.AISummary.Bullets = []string{}
	}
	if out.AISummary.Caveats == nil {
		out.AISummary.Caveats = []string{}
	}
	return out, promptLen, nil
}

func computeSplitFromLabels(txs []txForAI, labels []ConsumptionAnalysisLabelDTO) (AIAnalysisSplitDTO, AIAnalysisSplitDTO) {
	labelMap := map[string]ConsumptionAnalysisLabelDTO{}
	for _, l := range labels {
		labelMap[l.TransactionID] = l
	}
	rigidAmount := 0.0
	nonRigidAmount := 0.0
	rigidByCat := map[string]float64{}
	nonRigidByCat := map[string]float64{}
	for _, t := range txs {
		l, ok := labelMap[t.TransactionID]
		if !ok {
			continue
		}
		if l.Label == "rigid" {
			rigidAmount += t.Amount
			rigidByCat[t.CategoryL1] += t.Amount
		} else {
			nonRigidAmount += t.Amount
			nonRigidByCat[t.CategoryL1] += t.Amount
		}
	}
	total := rigidAmount + nonRigidAmount
	rigid := AIAnalysisSplitDTO{Amount: round2(rigidAmount)}
	nonRigid := AIAnalysisSplitDTO{Amount: round2(nonRigidAmount)}
	if total > 0 {
		rigid.Ratio = clamp01(round4(rigidAmount / total))
		nonRigid.Ratio = clamp01(round4(nonRigidAmount / total))
	}
	rigid.BreakdownL1 = mapToNameAmount(rigidByCat)
	nonRigid.BreakdownL1 = mapToNameAmount(nonRigidByCat)
	return rigid, nonRigid
}

func mapToNameAmount(input map[string]float64) []NameAmountDTO {
	items := make([]NameAmountDTO, 0, len(input))
	for k, v := range input {
		items = append(items, NameAmountDTO{Name: k, Amount: round2(v)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Amount > items[j].Amount })
	return items
}

func buildLabelStats(txs []txForAI, labels []ConsumptionAnalysisLabelDTO) labelStatsDTO {
	labelMap := map[string]ConsumptionAnalysisLabelDTO{}
	for _, l := range labels {
		labelMap[l.TransactionID] = l
	}
	stats := labelStatsDTO{TotalCount: len(txs)}
	byNonRigidCat := map[string]float64{}
	byWastefulCat := map[string]float64{}
	byWastefulMerchant := map[string]float64{}
	for _, t := range txs {
		l, ok := labelMap[t.TransactionID]
		if !ok {
			continue
		}
		if l.Label == "rigid" {
			stats.RigidCount++
		} else {
			stats.NonRigidCount++
			byNonRigidCat[t.CategoryL1] += t.Amount
		}
		switch l.Assessment {
		case "reasonable":
			stats.ReasonableCount++
		case "suspicious":
			stats.SuspiciousCount++
		case "wasteful":
			stats.WastefulCount++
			byWastefulCat[t.CategoryL1] += t.Amount
			byWastefulMerchant[t.Merchant] += t.Amount
		}
	}
	stats.TopNonRigidCategories = topNameAmount(byNonRigidCat, 8)
	stats.TopWastefulCategories = topNameAmount(byWastefulCat, 8)
	stats.TopWastefulMerchants = topNameAmount(byWastefulMerchant, 8)
	return stats
}

func topNameAmount(input map[string]float64, limit int) []NameAmountDTO {
	items := mapToNameAmount(input)
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func listTransactionsForAI(startDate, endDate, projectID string, limit int) ([]txForAI, error) {
	type row struct {
		ID              uint
		TransactionTime time.Time
		Merchant        string
		Amount          float64
		CategoryL1      string
		CategoryL2      string
		Remark          string
	}
	var rows []row

	query := db.DB.Table("transactions").
		Select("transactions.id as id, transactions.transaction_time as transaction_time, COALESCE(NULLIF(TRIM(transactions.merchant), ''), '未知') as merchant, transactions.amount as amount, category_l1.name as category_l1, category_l2.name as category_l2, COALESCE(transactions.remark, '') as remark").
		Joins("left join category_l1 on transactions.category_l1_id = category_l1.id").
		Joins("left join category_l2 on transactions.category_l2_id = category_l2.id").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	if err := query.Order("transactions.transaction_time asc").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]txForAI, 0, len(rows))
	for _, r := range rows {
		out = append(out, txForAI{
			TransactionID: strconv.FormatUint(uint64(r.ID), 10),
			Time:          r.TransactionTime.Format("2006-01-02 15:04:05"),
			Merchant:      r.Merchant,
			Amount:        round2(r.Amount),
			CategoryL1:    r.CategoryL1,
			CategoryL2:    r.CategoryL2,
			Remark:        r.Remark,
		})
	}
	return out, nil
}

func sumAmountInRangeForUser(startDate, endDate, projectID string) (float64, error) {
	query := db.DB.Table("transactions").
		Where("transactions.transaction_time BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	if strings.TrimSpace(projectID) != "" {
		query = query.Where("transactions.project_id = ?", projectID)
	}
	var total float64
	if err := query.Select("COALESCE(SUM(transactions.amount), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func getTrendSeriesForUser(startDate, endDate, trendType, projectID string) ([]TrendPointDTO, error) {
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
	if err := query.Group("date").Order("date").Scan(&rows).Error; err != nil {
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

func getTopCategoriesL1ForUser(startDate, endDate, projectID string, limit int) ([]NameAmountRatioDTO, error) {
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

func getTopMerchantsForUser(startDate, endDate, projectID string, limit int) ([]NameAmountDTO, error) {
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

func parseIntDefault(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}
