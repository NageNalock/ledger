package router_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"ledger/api/db"
	"ledger/api/model"
	"ledger/api/router"
	"ledger/api/utils"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", strings.Repeat("test-secret-", 4))
	var err error
	db.DB, err = gorm.Open(sqlite.Open("ledger.db?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB.DB()
	t.Cleanup(func() { sqlDB.Close() })
	err = db.DB.AutoMigrate(&model.User{}, &model.SystemSetting{}, &model.CategoryL1{}, &model.CategoryL2{}, &model.Transaction{}, &model.Project{}, &model.TransactionDraft{}, &model.AnalysisRun{}, &model.Budget{}, &model.BudgetLine{})
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	for i, name := range []string{"alice", "bob", "admin"} {
		role := "user"
		if name == "admin" {
			role = "admin"
		}
		if err = db.DB.Create(&model.User{ID: uint(i + 1), Username: name, PasswordHash: string(hash), Role: role}).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.DB.Create(&model.CategoryL1{ID: 1, Name: "餐饮"})
	db.DB.Create(&model.CategoryL2{ID: 1, ParentID: 1, Name: "午餐"})
	s := httptest.NewServer(router.SetupRouter())
	t.Cleanup(s.Close)
	return s
}

type basicTransport struct {
	username string
	password string
}

func (b *basicTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.SetBasicAuth(b.username, b.password)
	return http.DefaultTransport.RoundTrip(r)
}
func connect(t *testing.T, s *httptest.Server, user string) (*mcp.ClientSession, *basicTransport) {
	t.Helper()
	auth := &basicTransport{user, "test-password"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: s.URL + "/mcp", HTTPClient: &http.Client{Transport: auth}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session, auth
}
func call(t *testing.T, s *mcp.ClientSession, name string, args any, wantError bool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError != wantError {
		t.Fatalf("%s isError=%v content=%v", name, result.IsError, result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty content")
	}
	return result.Content[0].(*mcp.TextContent).Text
}

func TestMCPAuthenticationAndCRUD(t *testing.T) {
	s := setup(t)
	for _, credentials := range []string{"", "alice:wrong"} {
		req, _ := http.NewRequest("POST", s.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if credentials != "" {
			req.SetBasicAuth("alice", "wrong")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("expected 401, got %d", res.StatusCode)
		}
	}
	req, _ := http.NewRequest("POST", s.URL+"/mcp", nil)
	req.SetBasicAuth("alice", "test-password")
	req.Header.Set("Origin", "https://evil.example")
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("Origin accepted")
	}
	alice, _ := connect(t, s, "alice")
	bob, _ := connect(t, s, "bob")
	admin, _ := connect(t, s, "admin")
	catalog, err := alice.ListTools(context.Background(), nil)
	if err != nil || len(catalog.Tools) != 7 {
		t.Fatalf("catalog: %v %v", catalog, err)
	}
	call(t, alice, "list_categories", map[string]any{}, false)
	call(t, alice, "list_projects", map[string]any{}, false)
	input := map[string]any{"transaction_time": "2026-09-14T12:00:00+08:00", "merchant": "午餐", "amount": 25.5, "category_l1_id": 1, "category_l2_id": 1, "remark": "original"}
	data := call(t, alice, "create_transaction", input, false)
	var tx model.Transaction
	if err = json.Unmarshal([]byte(data), &tx); err != nil {
		t.Fatal(err)
	}
	if tx.UserID != 1 || tx.Amount != 25.5 {
		t.Fatalf("wrong entry: %s", data)
	}
	input["id"] = tx.ID
	input["amount"] = 30
	input["remark"] = ""
	call(t, bob, "get_transaction", map[string]any{"id": tx.ID}, true)
	call(t, bob, "update_transaction", input, true)
	call(t, bob, "delete_transaction", map[string]any{"id": tx.ID}, true)
	list := call(t, bob, "list_transactions", map[string]any{"user_id": 1}, false)
	var page struct {
		Total int `json:"total"`
	}
	json.Unmarshal([]byte(list), &page)
	if page.Total != 0 {
		t.Fatal("cross-user list leaked")
	}
	call(t, alice, "update_transaction", input, false)
	data = call(t, admin, "get_transaction", map[string]any{"id": tx.ID}, false)
	json.Unmarshal([]byte(data), &tx)
	if tx.Amount != 30 || tx.Remark != "" {
		t.Fatal("update failed")
	}
	input["category_l2_id"] = 999
	call(t, alice, "update_transaction", input, true)
	input["category_l2_id"] = 1
	input["project_id"] = 999
	call(t, alice, "update_transaction", input, true)
	call(t, admin, "delete_transaction", map[string]any{"id": tx.ID}, false)
	call(t, alice, "get_transaction", map[string]any{"id": tx.ID}, true)
	// Deleting/changing an account invalidates existing client connections.
	hash, _ := bcrypt.GenerateFromPassword([]byte("changed-password"), bcrypt.MinCost)
	db.DB.Model(&model.User{}).Where("id=1").Update("password_hash", string(hash))
	_, err = alice.ListTools(context.Background(), nil)
	if err == nil {
		t.Fatal("old password still accepted")
	}
	for _, deleted := range []bool{false, true} {
		if deleted {
			db.DB.Delete(&model.User{}, 1)
		}
		req, _ := http.NewRequest("POST", s.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{}}`))
		req.SetBasicAuth("alice", "changed-password")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 200
		if deleted {
			want = 401
		}
		if res.StatusCode != want {
			t.Fatalf("password/account refresh: %d", res.StatusCode)
		}
	}

}

func TestExportContainsRestorableDatabaseAndUploads(t *testing.T) {
	s := setup(t)
	os.Mkdir("uploads", 0700)
	os.WriteFile("uploads/avatar.png", []byte("test image bytes"), 0600)
	db.DB.Create(&model.SystemSetting{SettingKey: "openai_api_key", Value: "test-key"})
	os.WriteFile(".env", []byte("JWT_SECRET=must-not-be-exported"), 0600)
	db.DB.Create(&model.Project{ID: 1, Name: "test-project"})
	projectID := uint(1)
	db.DB.Create(&model.Transaction{UserID: 1, TransactionTime: time.Now(), Amount: 25, CategoryL1ID: 1, CategoryL2ID: 1, ProjectID: &projectID, ExternalID: "backup-test"})
	db.DB.Create(&model.TransactionDraft{UserID: 1, Merchant: "draft-test"})
	db.DB.Create(&model.AnalysisRun{ID: "test-analysis", UserID: 1, StartDate: "2026-09-01", EndDate: "2026-09-30"})
	db.DB.Create(&model.Budget{ID: "test-budget", Month: "2026-09"})
	db.DB.Create(&model.BudgetLine{BudgetID: "test-budget", Amount: 100})
	for _, id := range []uint{0, 1, 3} {
		req, _ := http.NewRequest("GET", s.URL+"/api/backup/export", nil)
		if id > 0 {
			token, _ := utils.GenerateToken(id, "test")
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(res.Body)
		res.Body.Close()
		expected := 200
		if id == 0 {
			expected = 401
		}
		if id == 1 {
			expected = 403
		}
		if res.StatusCode != expected {
			t.Fatalf("export %d: %d %s", id, res.StatusCode, content)
		}
		if id != 3 {
			continue
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("backup can be cached")
		}
		archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatal(err)
		}
		files := map[string][]byte{}
		for _, entry := range archive.File {
			r, _ := entry.Open()
			files[entry.Name], err = io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if string(files["uploads/avatar.png"]) != "test image bytes" {
			t.Fatal("missing uploads")
		}
		var manifest struct {
			SHA256 map[string]string `json:"sha256"`
		}
		json.Unmarshal(files["manifest.json"], &manifest)
		for name, data := range files {
			if name == "manifest.json" {
				continue
			}
			sum := sha256.Sum256(data)
			if manifest.SHA256[name] != hex.EncodeToString(sum[:]) {
				t.Fatal("checksum mismatch")
			}
		}
		snapshot := filepath.Join(t.TempDir(), "restored.db")
		os.WriteFile(snapshot, files["ledger.db"], 0600)
		restored, err := gorm.Open(sqlite.Open(snapshot), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := restored.DB()
		defer sqlDB.Close()
		var users []model.User
		restored.Find(&users)
		if len(users) != 3 || users[0].PasswordHash == "" {
			t.Fatal("users/password hashes not preserved")
		}
		var setting model.SystemSetting
		restored.First(&setting)
		if setting.Value != "test-key" {
			t.Fatal("settings not preserved")
		}
		for _, table := range []string{"transactions", "projects", "transaction_drafts", "analysis_runs", "budgets", "budget_lines", "category_l1", "category_l2"} {
			var count int64
			if err := restored.Table(table).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("restored table %s count=%d err=%v", table, count, err)
			}
		}
		if _, exists := files[".env"]; exists {
			t.Fatal("environment secrets included")
		}
		var check string
		restored.Raw("PRAGMA integrity_check").Scan(&check)
		if check != "ok" {
			t.Fatal(check)
		}
	}
}
