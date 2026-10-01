package router

import (
	"ledger/api/controller"
	"ledger/api/mcpserver"
	"ledger/api/middleware"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func corsAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS"))
	if raw == "" {
		return []string{"http://localhost:5173"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = []string{"http://localhost:5173"}
	}
	return out
}

func originAllowed(origin string, allow []string) bool {
	for _, a := range allow {
		if a == "*" {
			return true
		}
		if a == origin {
			return true
		}
	}
	return false
}

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestLogger(), middleware.RecoveryWithStack())

	// Static Files (Uploads)
	if strings.TrimSpace(os.Getenv("SERVE_UPLOADS")) == "1" {
		r.Static("/uploads", "./uploads")
	}

	// Static Files (Frontend)
	// Serve static files from "dist" directory
	r.Static("/assets", "./dist/assets")
	r.StaticFile("/favicon.ico", "./dist/favicon.ico")
	r.StaticFile("/logo.jpeg", "./dist/logo.jpeg")

	// Public, read-only discovery documents. MCP itself remains authenticated.
	r.StaticFile("/agent", "./dist/agent.html")
	r.StaticFile("/llms.txt", "./dist/llms.txt")
	serveSkill := func(file string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Header("Content-Type", "text/markdown; charset=utf-8")
			c.Header("X-Content-Type-Options", "nosniff")
			c.File(file)
		}
	}
	ledgerSkill := serveSkill("./dist/skills/ledger/SKILL.md")
	r.Match([]string{"GET", "HEAD"}, "/SKILL.md", ledgerSkill)
	r.Match([]string{"GET", "HEAD"}, "/skills/ledger/SKILL.md", ledgerSkill)
	r.Match([]string{"GET", "HEAD"}, "/skills/douyin-ledger/SKILL.md", serveSkill("./dist/skills/douyin-ledger/SKILL.md"))

	// SPA Fallback
	r.NoRoute(func(c *gin.Context) {
		// If request path starts with /api, return 404
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{"error": "Not Found"})
			return
		}
		// Otherwise serve index.html
		c.File("./dist/index.html")
	})

	allowOrigins := corsAllowedOrigins()
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && originAllowed(origin, allowOrigins) {
			if strings.Contains(os.Getenv("CORS_ALLOW_ORIGINS"), "*") {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Del("Access-Control-Allow-Credentials")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	mcpHandler := mcpserver.NewHandler()
	r.Any("/mcp", gin.WrapH(mcpHandler))

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", controller.Login)
			if strings.TrimSpace(os.Getenv("ENABLE_SIGNUP")) == "1" {
				auth.POST("/register", controller.Register)
			}
			auth.GET("/captcha", controller.GetCaptcha)
		}

		// Protected Routes
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.POST("/upload", controller.UploadFile)
			// Admin Routes
			protected.GET("/users", controller.GetUsers)
			protected.POST("/users", controller.CreateUser)
			protected.DELETE("/users/:id", controller.DeleteUser)
			protected.PUT("/users/profile", controller.UpdateUserProfile) // Self update

			protected.GET("/backup/export", controller.ExportBackup)

			protected.GET("/settings", controller.GetSystemSettings)
			protected.PUT("/settings", controller.UpdateSystemSettings)

			protected.GET("/categories", controller.GetCategories)
			protected.POST("/categories", controller.CreateCategory)
			protected.PUT("/categories", controller.UpdateCategory)
			protected.DELETE("/categories/:id", controller.DeleteCategory)

			protected.GET("/projects", controller.GetProjects)
			protected.GET("/projects/:id", controller.GetProject)
			protected.GET("/projects/:id/summary", controller.GetProjectSummary)
			protected.POST("/projects", controller.CreateProject)
			protected.PUT("/projects/:id", controller.UpdateProject)
			protected.DELETE("/projects/:id", controller.DeleteProject)

			protected.GET("/transactions", controller.GetTransactions)
			protected.POST("/transactions", controller.CreateTransaction)
			protected.PUT("/transactions/:id", controller.UpdateTransaction)
			protected.DELETE("/transactions/:id", controller.DeleteTransaction)

			protected.POST("/ai/classify", controller.ClassifyTransaction)
			protected.POST("/ai/consumption-analysis", controller.PostConsumptionAnalysisV2)
			protected.POST("/ai/budget-draft", controller.PostAIBudgetDraft)
			protected.POST("/ai/models", controller.PostAIModels)
			protected.POST("/ai/test", controller.PostAITest)
			protected.GET("/ai/consumption-analysis", controller.GetConsumptionAnalysis)
			protected.GET("/ai/consumption-analysis/history", controller.GetConsumptionAnalysisHistoryV2)
			protected.GET("/ai/consumption-analysis/:analysis_id", controller.GetConsumptionAnalysisDetailV2)

			// Import & Drafts
			protected.POST("/import/alipay", controller.ImportAlipay)
			protected.POST("/import/wechat", controller.ImportWechat)
			protected.GET("/drafts", controller.GetDrafts)
			protected.PUT("/drafts/:id", controller.UpdateDraft)
			protected.DELETE("/drafts/:id", controller.DeleteDraft)
			protected.POST("/drafts/:id/analyze", controller.AnalyzeDraft)
			protected.POST("/drafts/analyze", controller.BatchAnalyzeDrafts)
			protected.POST("/drafts/submit", controller.SubmitDrafts)

			// Reports
			protected.GET("/reports/trend", controller.GetTrendReport)
			protected.GET("/reports/category", controller.GetCategoryReport)
			protected.GET("/reports/merchant", controller.GetMerchantReport)

			// Budgets
			protected.GET("/budgets/active", controller.GetActiveBudget)
			protected.GET("/budgets/history", controller.ListBudgetHistory)
			protected.POST("/budgets", controller.CreateBudget)
			protected.PUT("/budgets/active", controller.UpsertActiveBudget)
			protected.POST("/budgets/active/discard", controller.DiscardActiveBudget)
		}
	}

	return r
}
