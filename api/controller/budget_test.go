package controller

import "testing"

func TestNormalizeMonth(t *testing.T) {
	got, err := normalizeMonth("2026-02")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "2026-02" {
		t.Fatalf("expected 2026-02, got %s", got)
	}

	got2, err := normalizeMonth("2026-02-28")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got2 != "2026-02" {
		t.Fatalf("expected 2026-02, got %s", got2)
	}

	_, err = normalizeMonth("2026/02")
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestEvalWaterline(t *testing.T) {
	thresholds := []budgetThresholdDTO{{Pct: 0.5, Label: "提醒"}, {Pct: 0.8, Label: "预警"}, {Pct: 1.0, Label: "超支"}}

	if got := evalWaterline(0.3, thresholds); got != "ok" {
		t.Fatalf("expected ok, got %s", got)
	}
	if got := evalWaterline(0.5, thresholds); got != "提醒" {
		t.Fatalf("expected 提醒, got %s", got)
	}
	if got := evalWaterline(0.81, thresholds); got != "预警" {
		t.Fatalf("expected 预警, got %s", got)
	}
	if got := evalWaterline(1.2, thresholds); got != "超支" {
		t.Fatalf("expected 超支, got %s", got)
	}
}
