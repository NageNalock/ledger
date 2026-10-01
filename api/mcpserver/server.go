// Package mcpserver exposes Ledger tools over authenticated Streamable HTTP.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"ledger/api/controller"
	"ledger/api/model"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type principalKey struct{}
type response struct {
	header http.Header
	bytes.Buffer
	status int
}

func (r *response) Header() http.Header { return r.header }
func (r *response) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}
func (r *response) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	return r.Buffer.Write(p)
}

// Use the same handlers and validation as the HTTP API, without a network hop,
// forwarding credentials, or minting long-lived tokens.
func toolAPI() *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		u, ok := c.Request.Context().Value(principalKey{}).(model.User)
		if !ok {
			c.AbortWithStatus(401)
			return
		}
		c.Set("userID", u.ID)
		c.Set("role", u.Role)
		c.Set("mcpScoped", true)
	})
	r.GET("/transactions", controller.GetTransactions)
	r.GET("/transactions/:id", controller.GetTransaction)
	r.POST("/transactions", controller.CreateTransaction)
	r.PUT("/transactions/:id", controller.UpdateTransaction)
	r.DELETE("/transactions/:id", controller.DeleteTransaction)
	r.GET("/categories", controller.GetCategories)
	r.GET("/projects", controller.GetProjects)
	return r
}

func invoke(ctx context.Context, api http.Handler, user model.User, method, path string, input any) (*mcp.CallToolResult, any, error) {
	var b []byte
	if input != nil {
		var err error
		b, err = json.Marshal(input)
		if err != nil {
			return nil, nil, err
		}
	}
	req, err := http.NewRequestWithContext(context.WithValue(ctx, principalKey{}, user), method, path, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	out := &response{header: make(http.Header)}
	api.ServeHTTP(out, req)
	log.Printf("mcp operation user_id=%d method=%s path=%s status=%d", user.ID, method, req.URL.Path, out.status)
	return &mcp.CallToolResult{IsError: out.status >= 400, Content: []mcp.Content{&mcp.TextContent{Text: out.String()}}}, nil, nil
}

type IDInput struct {
	ID uint `json:"id" jsonschema:"Transaction ID (positive integer)"`
}
type WriteInput struct {
	TransactionTime string  `json:"transaction_time" jsonschema:"RFC3339 timestamp with timezone, e.g. 2026-09-14T12:00:00+08:00"`
	Merchant        string  `json:"merchant,omitempty"`
	Amount          float64 `json:"amount" jsonschema:"Nonzero amount in currency units; positive means expense, negative means refund"`
	CategoryL1ID    uint    `json:"category_l1_id"`
	CategoryL2ID    uint    `json:"category_l2_id"`
	Source          string  `json:"source,omitempty"`
	Remark          string  `json:"remark,omitempty"`
	ProjectID       *uint   `json:"project_id,omitempty"`
}
type UpdateInput struct {
	ID uint `json:"id"`
	WriteInput
}
type ListInput struct {
	Page         int    `json:"page,omitempty"`
	PageSize     int    `json:"page_size,omitempty" jsonschema:"Maximum 100, default 20"`
	StartDate    string `json:"start_date,omitempty" jsonschema:"Inclusive date YYYY-MM-DD"`
	EndDate      string `json:"end_date,omitempty" jsonschema:"Inclusive date YYYY-MM-DD"`
	CategoryL1ID uint   `json:"category_l1_id,omitempty"`
	CategoryL2ID uint   `json:"category_l2_id,omitempty"`
	ProjectID    uint   `json:"project_id,omitempty"`
	Source       string `json:"source,omitempty"`
	UserID       uint   `json:"user_id,omitempty" jsonschema:"Administrator filter; does not grant access to another user's entries"`
}

func NewHandler() http.Handler {
	api := toolAPI()
	return authenticated(func(w http.ResponseWriter, r *http.Request, user model.User) {
		s := mcp.NewServer(&mcp.Implementation{Name: "ledger", Version: "1.0.0"}, &mcp.ServerOptions{Instructions: "Use list_categories and list_projects to resolve IDs before writing. New entries belong to the authenticated user. Ordinary users can only access their own entries; admins can access all. update_transaction replaces all editable fields: first get_transaction and preserve fields that were not requested to change. Amounts are currency units, positive for expenses and negative for refunds. Use explicit timezones. Do not automatically retry creates after an uncertain network failure; query first to avoid duplicate entries."})
		read := &mcp.ToolAnnotations{ReadOnlyHint: true}
		mcp.AddTool(s, &mcp.Tool{Name: "list_transactions", Description: "查询流水，支持日期、分类、项目、来源筛选和分页", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, any, error) {
			values := url.Values{}
			b, _ := json.Marshal(in)
			var args map[string]any
			decoder := json.NewDecoder(bytes.NewReader(b))
			decoder.UseNumber()
			_ = decoder.Decode(&args)
			for k, v := range args {
				values.Set(k, fmt.Sprint(v))
			}
			return invoke(ctx, api, user, "GET", "/transactions?"+values.Encode(), nil)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "get_transaction", Description: "按 ID 查询单笔流水", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
			if in.ID == 0 {
				return nil, nil, fmt.Errorf("id must be positive")
			}
			return invoke(ctx, api, user, "GET", "/transactions/"+strconv.FormatUint(uint64(in.ID), 10), nil)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "create_transaction", Description: "新增一笔流水，归属于当前登录账号"}, func(ctx context.Context, _ *mcp.CallToolRequest, in WriteInput) (*mcp.CallToolResult, any, error) {
			return invoke(ctx, api, user, "POST", "/transactions", in)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "update_transaction", Description: "完整替换流水可编辑字段；先读取原流水，保留未修改字段"}, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateInput) (*mcp.CallToolResult, any, error) {
			if in.ID == 0 {
				return nil, nil, fmt.Errorf("id must be positive")
			}
			return invoke(ctx, api, user, "PUT", "/transactions/"+strconv.FormatUint(uint64(in.ID), 10), in.WriteInput)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "delete_transaction", Description: "按 ID 永久删除一笔流水"}, func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, any, error) {
			if in.ID == 0 {
				return nil, nil, fmt.Errorf("id must be positive")
			}
			return invoke(ctx, api, user, "DELETE", "/transactions/"+strconv.FormatUint(uint64(in.ID), 10), nil)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "list_categories", Description: "查询共享的一二级分类及其 ID", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return invoke(ctx, api, user, "GET", "/categories", nil)
		})
		mcp.AddTool(s, &mcp.Tool{Name: "list_projects", Description: "查询共享项目及其 ID", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return invoke(ctx, api, user, "GET", "/projects", nil)
		})
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true}).ServeHTTP(w, r)
	})
}
