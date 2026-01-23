package controller

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	ai "ledger/api/ai"
	"ledger/api/db"
	"ledger/api/model"

	"github.com/gin-gonic/gin"
)

func requireAdmin(c *gin.Context) (*model.User, bool) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}

	var user model.User
	if err := db.DB.First(&user, userIDValue).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}
	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
		return nil, false
	}
	return &user, true
}

type aiModelsRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type aiModelsResponse struct {
	Models []string `json:"models"`
}

func PostAIModels(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}

	var input aiModelsRequest
	_ = c.ShouldBindJSON(&input)

	conf, err := getAIConfig()
	if err != nil {
		log.Printf("AI models request failed: config error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		baseURL = conf.BaseURL
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = conf.APIKey
	}
	apiKey = firstAPIKey(apiKey)

	log.Printf("AI models request: base_url=%s", baseURL)

	client := ai.NewClient(baseURL, apiKey)
	models, err := client.ListModels(c.Request.Context())
	if err != nil {
		log.Printf("AI models request failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("AI models request success: %d models found", len(models))
	sort.Strings(models)
	c.JSON(http.StatusOK, aiModelsResponse{Models: models})
}

type aiTestRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type aiTestResponse struct {
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Model     string `json:"model"`
	Snippet   string `json:"snippet,omitempty"`
}

func PostAITest(c *gin.Context) {
	if _, ok := requireAdmin(c); !ok {
		return
	}

	var input aiTestRequest
	_ = c.ShouldBindJSON(&input)

	conf, err := getAIConfig()
	if err != nil {
		log.Printf("AI test request failed: config error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		baseURL = conf.BaseURL
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = conf.APIKey
	}
	apiKey = firstAPIKey(apiKey)
	modelName := strings.TrimSpace(input.Model)
	if modelName == "" {
		modelName = conf.Model
	}

	log.Printf("AI test request: base_url=%s, model=%s", baseURL, modelName)

	client := ai.NewClient(baseURL, apiKey)

	start := time.Now()
	content, err := client.CreateChatCompletion(c.Request.Context(), ai.ChatCompletionRequest{
		Model: modelName,
		Messages: []ai.Message{
			{Role: "user", Content: "Reply with exactly: OK"},
		},
	})
	if err != nil {
		log.Printf("AI test request failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snippet := stripCodeFence(content)
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}

	log.Printf("AI test request success: latency=%vms", time.Since(start).Milliseconds())

	c.JSON(http.StatusOK, aiTestResponse{
		OK:        true,
		LatencyMs: time.Since(start).Milliseconds(),
		Model:     modelName,
		Snippet:   snippet,
	})
}
