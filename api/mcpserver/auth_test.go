package mcpserver

import (
	"testing"
	"time"
)

func TestRateLimitAndExpiry(t *testing.T) {
	l := &limiter{entries: map[string]attempt{}}
	for i := 0; i < 120; i++ {
		if !l.allow("127.0.0.1") {
			t.Fatal("limited too early")
		}
	}
	if l.allow("127.0.0.1") {
		t.Fatal("limit not enforced")
	}
	if !l.allow("127.0.0.2") {
		t.Fatal("unrelated IP limited")
	}
	l.entries["127.0.0.1"] = attempt{count: 120, until: time.Now().Add(-time.Second)}
	if !l.allow("127.0.0.1") {
		t.Fatal("expired limit not cleared")
	}
}
