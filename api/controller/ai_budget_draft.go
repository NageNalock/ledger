package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"ledger/api/db"
	"ledger/api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type aiBudgetDraftRequest struct {
	AnalysisID string `json:"analysis_id" binding:"required"`
	Month      string `json:"month"`
}

type aiBudgetDraftResponse struct {
	Month       string            `json:"month"`
	TotalAmount float64           `json:"total_amount"`
	Lines       []budgetLineInput `json:"lines"`
}

type analysisResultForBudget struct {
	Range struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	} `json:"range"`
	Kpis struct {
		TotalAmount float64 `json:"total_amount"`
	} `json:"kpis"`
	Evidence struct {
		TopCategoriesL1 []struct {
			Name   string  `json:"name"`
			Amount float64 `json:"amount"`
			Ratio  float64 `json:"ratio"`
		} `json:"top_categories_l1"`
	} `json:"evidence"`
	BudgetSuggestion struct {
		NextPeriodTotal *struct {
			Min float64 `json:"min"`
			Max float64 `json:"max"`
		} `json:"next_period_total"`
	} `json:"budget_suggestion"`
}

func PostAIBudgetDraft(c *gin.Context) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, _ := userIDValue.(uint)

	var input aiBudgetDraftRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	analysisID := strings.TrimSpace(input.AnalysisID)
	if analysisID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "analysis_id required"})
		return
	}

	var run model.AnalysisRun
	if err := db.DB.Where("id = ?", analysisID).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if run.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if strings.TrimSpace(run.Status) != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "analysis not completed"})
		return
	}
	if strings.TrimSpace(run.ResultJSON) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "analysis result empty"})
		return
	}

	var result analysisResultForBudget
	if err := json.Unmarshal([]byte(run.ResultJSON), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid analysis result"})
		return
	}

	month := strings.TrimSpace(input.Month)
	if month == "" {
		if len(result.Range.EndDate) >= 7 {
			month = result.Range.EndDate[:7]
		}
	}
	m, err := normalizeMonth(month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
		return
	}

	total := 0.0
	if result.BudgetSuggestion.NextPeriodTotal != nil {
		minv := result.BudgetSuggestion.NextPeriodTotal.Min
		maxv := result.BudgetSuggestion.NextPeriodTotal.Max
		if minv > 0 && maxv > 0 {
			total = (minv + maxv) / 2
		} else if maxv > 0 {
			total = maxv
		} else {
			total = minv
		}
	}
	if total <= 0 {
		total = result.Kpis.TotalAmount
	}
	total = round2(total)
	if total <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot infer total"})
		return
	}

	tops := make([]struct {
		Name  string
		Ratio float64
	}, 0, len(result.Evidence.TopCategoriesL1))
	for _, x := range result.Evidence.TopCategoriesL1 {
		name := strings.TrimSpace(x.Name)
		if name == "" {
			continue
		}
		r := x.Ratio
		if r <= 0 {
			continue
		}
		tops = append(tops, struct {
			Name  string
			Ratio float64
		}{Name: name, Ratio: r})
	}
	sort.Slice(tops, func(i, j int) bool { return tops[i].Ratio > tops[j].Ratio })

	maxN := 8
	minRatio := 0.03
	lines := make([]budgetLineInput, 0, maxN)
	for _, t := range tops {
		if len(lines) >= maxN {
			break
		}
		if t.Ratio < minRatio {
			continue
		}
		var c1 model.CategoryL1
		if err := db.DB.Where("name = ?", t.Name).First(&c1).Error; err != nil {
			continue
		}
		id := c1.ID
		lines = append(lines, budgetLineInput{CategoryL1ID: &id, CategoryL2ID: nil, Amount: round2(total * t.Ratio)})
	}

	out := aiBudgetDraftResponse{Month: m, TotalAmount: total, Lines: lines}
	c.JSON(http.StatusOK, out)
}
