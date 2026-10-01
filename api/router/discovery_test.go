package router_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryIsPublicButDataRequiresAuthentication(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(root, "..", "..", "public")
	skill, err := os.ReadFile(filepath.Join(public, "skills", "ledger", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	douyin, err := os.ReadFile(filepath.Join(public, "skills", "douyin-ledger", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(public, "llms.txt"))
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join(root, "..", "..", "src", "content", "agent-access.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := setup(t)
	if err = os.MkdirAll("dist/skills/ledger", 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll("dist/skills/douyin-ledger", 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"skills/ledger/SKILL.md": skill, "skills/douyin-ledger/SKILL.md": douyin, "llms.txt": index, "agent.html": guide} {
		if err = os.WriteFile(filepath.Join("dist", name), body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string][]byte{"/agent": guide, "/llms.txt": index, "/SKILL.md": skill, "/skills/ledger/SKILL.md": skill, "/skills/douyin-ledger/SKILL.md": douyin} {
		res, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 || string(actual) != string(body) {
			t.Fatalf("public document %s: status=%d err=%v", path, res.StatusCode, err)
		}
		if strings.HasSuffix(path, "SKILL.md") && !strings.HasPrefix(res.Header.Get("Content-Type"), "text/markdown") {
			t.Fatal("skill served with incorrect content type")
		}
	}
	for _, path := range []string{"/api/transactions", "/api/settings", "/api/backup/export"} {
		res, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("data endpoint %s accessible anonymously: %d", path, res.StatusCode)
		}
	}
	req, _ := http.NewRequest("POST", server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("MCP is not protected")
	}
}
