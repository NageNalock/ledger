package controller

import "testing"

func TestComputeSplitFromLabels(t *testing.T) {
	txs := []txForAI{
		{TransactionID: "1", Amount: 100, CategoryL1: "餐饮"},
		{TransactionID: "2", Amount: 50, CategoryL1: "交通"},
		{TransactionID: "3", Amount: 80, CategoryL1: "餐饮"},
	}
	labels := []ConsumptionAnalysisLabelDTO{
		{TransactionID: "1", Label: "rigid", Assessment: "reasonable"},
		{TransactionID: "2", Label: "non_rigid", Assessment: "wasteful"},
		{TransactionID: "3", Label: "rigid", Assessment: "reasonable"},
	}

	rigid, nonRigid := computeSplitFromLabels(txs, labels)
	if rigid.Amount != 180 {
		t.Fatalf("rigid amount expected 180 got %v", rigid.Amount)
	}
	if nonRigid.Amount != 50 {
		t.Fatalf("non rigid amount expected 50 got %v", nonRigid.Amount)
	}
	if len(rigid.BreakdownL1) == 0 || rigid.BreakdownL1[0].Name != "餐饮" {
		t.Fatalf("rigid breakdown expected 餐饮")
	}
}

func TestBuildLabelStats(t *testing.T) {
	txs := []txForAI{
		{TransactionID: "1", Amount: 100, CategoryL1: "餐饮", Merchant: "A"},
		{TransactionID: "2", Amount: 50, CategoryL1: "交通", Merchant: "B"},
		{TransactionID: "3", Amount: 80, CategoryL1: "餐饮", Merchant: "C"},
	}
	labels := []ConsumptionAnalysisLabelDTO{
		{TransactionID: "1", Label: "rigid", Assessment: "reasonable"},
		{TransactionID: "2", Label: "non_rigid", Assessment: "wasteful"},
		{TransactionID: "3", Label: "non_rigid", Assessment: "suspicious"},
	}
	stats := buildLabelStats(txs, labels)
	if stats.TotalCount != 3 || stats.RigidCount != 1 || stats.NonRigidCount != 2 {
		t.Fatalf("unexpected counts: %+v", stats)
	}
	if stats.WastefulCount != 1 {
		t.Fatalf("wasteful count expected 1 got %d", stats.WastefulCount)
	}
	if len(stats.TopWastefulMerchants) == 0 || stats.TopWastefulMerchants[0].Name != "B" {
		t.Fatalf("expected top wasteful merchant B")
	}
}
