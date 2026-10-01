package mcpserver

import (
	"golang.org/x/crypto/bcrypt"
	"ledger/api/db"
	"ledger/api/model"
	"net"
	"net/http"
	"sync"
	"time"
)

type attempt struct {
	count int
	until time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]attempt
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, v := range l.entries {
		if now.After(v.until) {
			delete(l.entries, k)
		}
	}
	a := l.entries[key]
	if a.count >= 120 || (a.count == 0 && len(l.entries) >= 10000) {
		return false
	}
	if a.count == 0 {
		a.until = now.Add(time.Minute)
	}
	a.count++
	l.entries[key] = a
	return true
}

// Each HTTP request is authenticated afresh; no identity is cached in MCP sessions.
func authenticated(next func(http.ResponseWriter, *http.Request, model.User)) http.Handler {
	limits := &limiter{entries: make(map[string]attempt)}
	dummy, _ := bcrypt.GenerateFromPassword([]byte("unused-dummy-password"), bcrypt.DefaultCost)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// Non-browser clients omit Origin. Browser access is intentionally disabled.
		if r.Header.Get("Origin") != "" {
			http.Error(w, "Browser origins are not allowed", 403)
			return
		}
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !limits.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests", 429)
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok || username == "" || len(username) > 200 || len(password) > 72 {
			w.Header().Set("WWW-Authenticate", `Basic realm="ledger-mcp", charset="UTF-8"`)
			http.Error(w, "Invalid credentials", 401)
			return
		}
		var user model.User
		err = db.DB.WithContext(r.Context()).Where("username = ?", username).First(&user).Error
		hash := []byte(user.PasswordHash)
		if err != nil {
			hash = dummy
		}
		match := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
		if err != nil || !match {
			w.Header().Set("WWW-Authenticate", `Basic realm="ledger-mcp", charset="UTF-8"`)
			http.Error(w, "Invalid credentials", 401)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next(w, r, user)
	})
}
