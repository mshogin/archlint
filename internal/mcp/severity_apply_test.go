package mcp

import (
	"strings"
	"testing"
)

// TestApplySeverity — обогащение Violation severity-классом + флагами соундности
// из SSOT (Горизонт 1, объяснимость агентского гейта). Покрывает критерий:
// ERROR отличимо от WARNING/INFO; dead-code -> HumanInLoop; OpenWorld-kind ->
// RequiresDelta; незарегистрированный kind -> Severity="".
func TestApplySeverity(t *testing.T) {
	vs := []Violation{
		{Kind: "dead-code"},            // 0: ERROR open-world, human-in-loop
		{Kind: "circular-dependency"},  // 1: ERROR closed-world (без флагов)
		{Kind: "isp-usage-subset"},     // 2: ERROR requires-delta, НЕ human-in-loop
		{Kind: "structural-clone"},     // 3: WARNING / DRY
		{Kind: "srp-too-many-fields"},  // 4: INFO / SRP
		{Kind: "god-class"},            // 5: INFO / coupling-cohesion
		{Kind: "layer-violation"},      // 6: ERROR / layering
		{Kind: "totally-unknown-kind"}, // 7: не зареган -> Severity ""
	}
	ApplySeverity(vs)

	// dead-code: ERROR + все флаги условной соундности + principle reachability.
	if d := vs[0]; d.Severity != "ERROR" || !d.OpenWorld || !d.RequiresDelta || !d.HumanInLoop || d.Principle != "reachability" {
		t.Errorf("dead-code: %+v", d)
	}
	// dead-code remediation ОБЯЗАН содержать human-in-loop оговорку.
	if !strings.Contains(vs[0].Remediation, "человек") {
		t.Errorf("dead-code remediation без human-in-loop оговорки: %q", vs[0].Remediation)
	}
	// Каждое ИЗВЕСТНОЕ нарушение несёт actionable Remediation-направление.
	for idx := 0; idx < 7; idx++ { // 0..6 — зарегистрированные kinds
		if vs[idx].Remediation == "" {
			t.Errorf("kind %q без Remediation", vs[idx].Kind)
		}
	}
	// circular: ERROR closed-world (флаги false) + acyclic-dependencies.
	if c := vs[1]; c.Severity != "ERROR" || c.OpenWorld || c.RequiresDelta || c.HumanInLoop || c.Principle != "acyclic-dependencies" {
		t.Errorf("circular: %+v", c)
	}
	// isp-usage-subset: ERROR, RequiresDelta=true, HumanInLoop=false (irritation, не destruction).
	if i := vs[2]; i.Severity != "ERROR" || !i.RequiresDelta || i.HumanInLoop || i.Principle != "ISP" {
		t.Errorf("isp: %+v", i)
	}
	// structural-clone: WARNING / DRY.
	if s := vs[3]; s.Severity != "WARNING" || s.Principle != "DRY" {
		t.Errorf("structural-clone: %+v", s)
	}
	// srp-too-many-fields: INFO / SRP.
	if s := vs[4]; s.Severity != "INFO" || s.Principle != "SRP" {
		t.Errorf("srp: %+v", s)
	}
	// god-class: INFO / coupling-cohesion.
	if g := vs[5]; g.Severity != "INFO" || g.Principle != "coupling-cohesion" {
		t.Errorf("god-class: %+v", g)
	}
	// layer-violation: ERROR / layering.
	if l := vs[6]; l.Severity != "ERROR" || l.Principle != "layering" {
		t.Errorf("layer: %+v", l)
	}
	// Незарегистрированный kind -> Severity "" (omitempty), principle "".
	if u := vs[7]; u.Severity != "" {
		t.Errorf("unknown kind должен иметь Severity \"\", got %q", u.Severity)
	}
}

// TestApplySeverity_DoesNotAffectFingerprint — severity/флаги АДДИТИВНЫ: дельта-
// идентичность (Fingerprint) НЕ меняется от обогащения (как Location/Message).
func TestApplySeverity_DoesNotAffectFingerprint(t *testing.T) {
	v := Violation{Kind: "circular-dependency", Message: "Circular (SCC): a <-> b", Anchor: "scc:a,b"}
	before := Fingerprint(v)
	vs := []Violation{v}
	ApplySeverity(vs)
	after := Fingerprint(vs[0])
	if before != after {
		t.Errorf("Fingerprint изменился от ApplySeverity: %q -> %q", before, after)
	}
	// sanity: обогащение реально произошло (severity + remediation заполнены),
	// но Fingerprint остался прежним — поля АДДИТИВНЫ, не в идентичности.
	if vs[0].Severity == "" || vs[0].Remediation == "" {
		t.Error("severity/remediation должны быть заполнены (sanity)")
	}
}
