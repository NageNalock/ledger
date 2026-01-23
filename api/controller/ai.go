package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	ai "ledger/api/ai"
	"ledger/api/db"
	dbModel "ledger/api/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AIClassifyInput struct {
	Merchant        string  `json:"merchant"`
	Amount          float64 `json:"amount"`
	TransactionTime string  `json:"transaction_time"`
	Remark          string  `json:"remark"`
}

type AIClassifyOutput struct {
	CategoryL1 string `json:"category_l1"`
	CategoryL2 string `json:"category_l2"`
	Reason     string `json:"reason,omitempty"`
}

func ClassifyTransaction(c *gin.Context) {
	var input AIClassifyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var l1s []dbModel.CategoryL1
	db.DB.Find(&l1s)

	categoriesContext := "Available Categories:\n"
	for _, l1 := range l1s {
		if l1.Name == "其他支出" {
			continue
		}
		var l2s []dbModel.CategoryL2
		db.DB.Where("parent_id = ?", l1.ID).Find(&l2s)
		subs := []string{}
		for _, l2 := range l2s {
			subs = append(subs, l2.Name)
		}
		categoriesContext += fmt.Sprintf("- %s: [%v]\n", l1.Name, subs)
	}

	prompt := fmt.Sprintf(`
You are a helpful assistant that classifies financial transactions.
%s

Classify the following transaction into one of the available Level 1 and Level 2 categories.
Transaction Details:
- Merchant: %s
- Amount: %.2f
- Time: %s
- Remark: %s

Return ONLY a JSON object with keys "category_l1" and "category_l2". Do not output markdown or explanations.
Example: {"category_l1": "餐饮美食", "category_l2": "午餐"}
`, categoriesContext, input.Merchant, input.Amount, input.TransactionTime, input.Remark)

	conf, err := getAIConfig()
	if err != nil {
		log.Printf("AI classify failed: config error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI Configuration Missing"})
		return
	}

	log.Printf("AI classify request: merchant=%s, amount=%.2f, model=%s", input.Merchant, input.Amount, conf.Model)

	client := ai.NewClient(conf.BaseURL, conf.APIKey)
	start := time.Now()
	content, err := client.CreateChatCompletion(c.Request.Context(), ai.ChatCompletionRequest{
		Model: conf.Model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		log.Printf("AI classify failed after %v: %v", time.Since(start), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI Service Error: " + err.Error()})
		return
	}

	log.Printf("AI classify success: latency=%vms", time.Since(start).Milliseconds())
	content = stripCodeFence(content)

	var output AIClassifyOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		log.Printf("AI classify parse failed: %v, content=%s", err, content)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response", "raw": content})
		return
	}

	c.JSON(http.StatusOK, output)
}

func AnalyzeWithAI(merchant string, amount float64, timeStr string, remark string) (string, string, error) {
	var l1s []dbModel.CategoryL1
	db.DB.Find(&l1s)

	categoriesContext := "Available Categories:\n"
	for _, l1 := range l1s {
		if l1.Name == "其他支出" {
			continue
		}
		var l2s []dbModel.CategoryL2
		db.DB.Where("parent_id = ?", l1.ID).Find(&l2s)
		subs := []string{}
		for _, l2 := range l2s {
			subs = append(subs, l2.Name)
		}
		categoriesContext += fmt.Sprintf("- %s: [%v]\n", l1.Name, subs)
	}

	prompt := fmt.Sprintf(`
You are a helpful assistant that classifies financial transactions.
%s

Classify the following transaction into one of the available Level 1 and Level 2 categories.
Transaction Details:
- Merchant: %s
- Amount: %.2f
- Time: %s
- Remark: %s

Return ONLY a JSON object with keys "category_l1" and "category_l2". 
If you cannot determine the category, or the transaction is unclear, return "unknown" for both fields, and include a "reason" field explaining why.
Do not output markdown or explanations.
Example Success: {"category_l1": "餐饮美食", "category_l2": "午餐"}
Example Failure: {"category_l1": "unknown", "category_l2": "unknown", "reason": "无法识别商家或商品信息"}
`, categoriesContext, merchant, amount, timeStr, remark)

	conf, err := getAIConfig()
	if err != nil {
		log.Printf("AI analyze failed: config error: %v", err)
		return "", "", fmt.Errorf("AI configuration missing")
	}

	log.Printf("AI analyze request: merchant=%s, amount=%.2f, model=%s", merchant, amount, conf.Model)

	client := ai.NewClient(conf.BaseURL, conf.APIKey)
	start := time.Now()
	content, err := client.CreateChatCompletion(context.Background(), ai.ChatCompletionRequest{
		Model: conf.Model,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		log.Printf("AI analyze failed after %v: %v", time.Since(start), err)
		return "", "", fmt.Errorf("AI service error: %v", err)
	}

	log.Printf("AI analyze success: latency=%vms", time.Since(start).Milliseconds())
	content = stripCodeFence(content)

	var output AIClassifyOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		log.Printf("AI analyze parse failed: %v, content=%s", err, content)
		return "", "", fmt.Errorf("Failed to parse AI response: %v. Raw: %s", err, content)
	}

	if output.CategoryL1 == "unknown" || output.CategoryL2 == "unknown" {
		reason := output.Reason
		if reason == "" {
			reason = "AI 无法确定分类"
		}
		log.Printf("AI analyze returned unknown: reason=%s", reason)
		return "", "", fmt.Errorf("%s", reason)
	}

	return output.CategoryL1, output.CategoryL2, nil
}
