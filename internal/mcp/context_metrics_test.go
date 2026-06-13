package mcp

import (
	"math"
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
)

// Golden context INFO-дескрипторов (порт complexity/coupling/depth).
// checkout{cart,payment} + catalog{search,cart}: complexity/depth max=2;
// coupling = |{cart}| / min(2,2) = 0.5.
func TestContextSignals(t *testing.T) {
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{
		{Name: "checkout", Components: []string{"app/cart", "app/payment"}},
		{Name: "catalog", Components: []string{"app/search", "app/cart"}},
	}}

	cs := ComputeContextSignals(cfg)
	if cs == nil {
		t.Fatal("ожидались context-сигналы")
	}

	if cs.MaxComplexity != 2 || cs.MaxDepth != 2 {
		t.Errorf("complexity/depth: got %d/%d, want 2/2", cs.MaxComplexity, cs.MaxDepth)
	}
	if math.Abs(cs.MaxCoupling-0.5) > 1e-9 {
		t.Errorf("coupling: got %v, want 0.5 (|{cart}|/min(2,2))", cs.MaxCoupling)
	}
	if cs.PerContext["checkout"] != 2 || cs.PerContext["catalog"] != 2 {
		t.Errorf("perContext: got %v", cs.PerContext)
	}
}

// Без контекстов -> nil (метрики неактивны).
func TestContextSignals_Inactive(t *testing.T) {
	if cs := ComputeContextSignals(&archlintcfg.Config{}); cs != nil {
		t.Errorf("без контекстов -> nil, got %+v", cs)
	}
}

// SPOF: "app/core" во ВСЕХ 3 контекстах -> single point of failure (WARNING DR-0060).
func TestContextSignals_SPOF(t *testing.T) {
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{
		{Name: "a", Components: []string{"app/core", "app/x"}},
		{Name: "b", Components: []string{"app/core", "app/y"}},
		{Name: "c", Components: []string{"app/core", "app/z"}},
	}}

	cs := ComputeContextSignals(cfg)
	if len(cs.SinglePointsOfFailure) != 1 || cs.SinglePointsOfFailure[0] != "app/core" {
		t.Fatalf("SPOF: ожидался [app/core], got %v", cs.SinglePointsOfFailure)
	}
	if cs.NearSPOFCount != 0 {
		t.Errorf("nearSPOF: got %d, want 0", cs.NearSPOFCount)
	}
}

// near-SPOF: "wide" в 4 из 5 контекстов (>=80%, не все) -> near, не SPOF.
func TestContextSignals_NearSPOF(t *testing.T) {
	c := func(name string, comps ...string) archlintcfg.ContextDef {
		return archlintcfg.ContextDef{Name: name, Components: comps}
	}
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{
		c("a", "wide", "1"), c("b", "wide", "2"), c("c", "wide", "3"), c("d", "wide", "4"), c("e", "5"),
	}}

	cs := ComputeContextSignals(cfg)
	if len(cs.SinglePointsOfFailure) != 0 {
		t.Errorf("SPOF: got %v, want []", cs.SinglePointsOfFailure)
	}
	if cs.NearSPOFCount != 1 {
		t.Errorf("nearSPOF: got %d, want 1 (wide в 4/5)", cs.NearSPOFCount)
	}
}
