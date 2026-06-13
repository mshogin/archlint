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
